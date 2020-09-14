package api_server

import (
	"context"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/jpillora/backoff"
	"go.uber.org/zap"

	"github.com/pingcap/tipocket/pkg/cluster/manager/types"
)

// PollPendingClusterRequests polls pending cluster requests
func (m *Manager) PollPendingClusterRequests(ctx context.Context) {
	b := &backoff.Backoff{
		Min:    1 * time.Second,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}
	for {
		time.Sleep(b.Duration())
		select {
		case <-ctx.Done():
			return
		default:
		}
		err := m.Resource.DB.Transaction(func(tx *gorm.DB) error {
			crs, err := m.Cluster.FindClusterRequests(tx, "status = ?", types.ClusterRequestStatusPending)
			if err != nil {
				return err
			}
			for _, cr := range crs {
				rrs, err := m.Resource.FindResourceRequests(tx, "id = ?", cr.RRID)
				if err != nil {
					return err
				}
				if len(rrs) != 1 {
					zap.L().Error("resource request is not found", zap.Uint("id", cr.RRID))
					continue
				}
				rr := rrs[0]
				if rr.Status != types.ResourceRequestStatusIdle {
					continue
				}
				rr.CRID = cr.ID
				rr.Status = types.ResourceRequestStatusPending
				// don't update cluster request here because this cluster request is still in pending until the
				// resource requests getting enough resources
				if err := m.Resource.UpdateResourceRequest(tx, rr); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			zap.L().Error("find cluster requests failed", zap.Error(err))
			continue
		}
		b.Reset()
	}
}
