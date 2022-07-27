package correlations

import (
	"context"

	"github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/util"
)

// createCorrelation adds a correlation
func (s CorrelationsService) createCorrelation(ctx context.Context, cmd CreateCorrelationCommand) (Correlation, error) {
	correlation := Correlation{
		UID:         util.GenerateShortUID(),
		SourceUID:   cmd.SourceUID,
		TargetUID:   cmd.TargetUID,
		Label:       cmd.Label,
		Description: cmd.Description,
	}

	err := s.SQLStore.WithTransactionalDbSession(ctx, func(session *sqlstore.DBSession) error {
		var err error

		query := &datasources.GetDataSourceQuery{
			OrgId: cmd.OrgId,
			Uid:   cmd.SourceUID,
		}
		if err = s.DataSourceService.GetDataSource(ctx, query); err != nil {
			return ErrSourceDataSourceDoesNotExists
		}

		if !cmd.SkipReadOnlyCheck && query.Result.ReadOnly {
			return ErrSourceDataSourceReadOnly
		}

		if err = s.DataSourceService.GetDataSource(ctx, &datasources.GetDataSourceQuery{
			OrgId: cmd.OrgId,
			Uid:   cmd.TargetUID,
		}); err != nil {
			return ErrTargetDataSourceDoesNotExists
		}

		_, err = session.Insert(correlation)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return Correlation{}, err
	}

	return correlation, nil
}

func (s CorrelationsService) deleteCorrelationsBySourceUID(ctx context.Context, cmd DeleteCorrelationsBySourceUIDCommand) error {
	return s.SQLStore.WithDbSession(ctx, func(session *sqlstore.DBSession) error {
		_, err := session.Delete(&Correlation{SourceUID: cmd.SourceUID})
		return err
	})
}

func (s CorrelationsService) deleteCorrelationsByTargetUID(ctx context.Context, cmd DeleteCorrelationsByTargetUIDCommand) error {
	return s.SQLStore.WithDbSession(ctx, func(session *sqlstore.DBSession) error {
		_, err := session.Delete(&Correlation{TargetUID: cmd.TargetUID})
		return err
	})
}
