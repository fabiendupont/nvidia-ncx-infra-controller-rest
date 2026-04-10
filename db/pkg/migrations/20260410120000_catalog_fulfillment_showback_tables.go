package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		tx, terr := db.BeginTx(ctx, &sql.TxOptions{})
		if terr != nil {
			handlePanic(terr, "failed to begin transaction")
		}

		// Create blueprint table
		_, err := tx.NewCreateTable().Model((*model.Blueprint)(nil)).IfNotExists().Exec(ctx)
		handleError(tx, err)

		// Create catalog_order table
		_, err = tx.NewCreateTable().Model((*model.CatalogOrder)(nil)).IfNotExists().Exec(ctx)
		handleError(tx, err)

		// Create catalog_service table
		_, err = tx.NewCreateTable().Model((*model.CatalogService)(nil)).IfNotExists().Exec(ctx)
		handleError(tx, err)

		// Create usage_record table
		_, err = tx.NewCreateTable().Model((*model.UsageRecord)(nil)).IfNotExists().Exec(ctx)
		handleError(tx, err)

		// Add indexes
		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_blueprint_tenant_id ON blueprint (tenant_id) WHERE tenant_id IS NOT NULL")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_blueprint_visibility ON blueprint (visibility)")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_blueprint_is_active ON blueprint (is_active)")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_catalog_order_tenant_id ON catalog_order (tenant_id)")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_catalog_order_status ON catalog_order (status)")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_catalog_service_tenant_id ON catalog_service (tenant_id)")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_catalog_service_order_id ON catalog_service (order_id)")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_usage_record_tenant_id ON usage_record (tenant_id)")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_usage_record_resource_id ON usage_record (resource_id)")
		handleError(tx, err)

		_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_usage_record_service_id ON usage_record (service_id)")
		handleError(tx, err)

		terr = tx.Commit()
		if terr != nil {
			handlePanic(terr, "failed to commit transaction")
		}

		fmt.Print(" [up migration] Created blueprint, catalog_order, catalog_service, usage_record tables. ")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		tx, terr := db.BeginTx(ctx, &sql.TxOptions{})
		if terr != nil {
			handlePanic(terr, "failed to begin transaction")
		}

		_, err := tx.NewDropTable().Model((*model.UsageRecord)(nil)).IfExists().Exec(ctx)
		handleError(tx, err)

		_, err = tx.NewDropTable().Model((*model.CatalogService)(nil)).IfExists().Exec(ctx)
		handleError(tx, err)

		_, err = tx.NewDropTable().Model((*model.CatalogOrder)(nil)).IfExists().Exec(ctx)
		handleError(tx, err)

		_, err = tx.NewDropTable().Model((*model.Blueprint)(nil)).IfExists().Exec(ctx)
		handleError(tx, err)

		terr = tx.Commit()
		if terr != nil {
			handlePanic(terr, "failed to commit transaction")
		}

		fmt.Print(" [down migration] Dropped blueprint, catalog_order, catalog_service, usage_record tables. ")
		return nil
	})
}
