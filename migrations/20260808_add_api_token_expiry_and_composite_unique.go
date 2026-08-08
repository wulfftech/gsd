package migrations

import (
	"context"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type AddAPITokenExpiryAndCompositeUnique20260808 struct{}

func (m AddAPITokenExpiryAndCompositeUnique20260808) ID() string {
	return "20260808_add_api_token_expiry_and_composite_unique"
}

func (m AddAPITokenExpiryAndCompositeUnique20260808) Description() string {
	return "Add expiry and last_used tracking to API tokens, change name unique constraint to composite (user_id, name)"
}

func (m AddAPITokenExpiryAndCompositeUnique20260808) Down(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// Drop new columns
		if tx.Migrator().HasColumn("api_tokens", "expires_at") {
			if err := tx.Migrator().DropColumn("api_tokens", "expires_at"); err != nil {
				log.Errorf("Failed to drop expires_at column: %v", err)
				return err
			}
		}

		if tx.Migrator().HasColumn("api_tokens", "last_used_at") {
			if err := tx.Migrator().DropColumn("api_tokens", "last_used_at"); err != nil {
				log.Errorf("Failed to drop last_used_at column: %v", err)
				return err
			}
		}

		// Drop composite index and recreate global unique index
		if tx.Migrator().HasIndex("api_tokens", "idx_user_token_name") {
			if err := tx.Migrator().DropIndex("api_tokens", "idx_user_token_name"); err != nil {
				log.Errorf("Failed to drop composite index: %v", err)
				return err
			}
		}

		// Recreate the old global unique index on name
		if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_name ON api_tokens(name)").Error; err != nil {
			log.Errorf("Failed to create global unique index on name: %v", err)
			return err
		}

		return nil
	})
}

func (m AddAPITokenExpiryAndCompositeUnique20260808) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// Add new columns if they don't exist
		if !tx.Migrator().HasColumn("api_tokens", "expires_at") {
			if err := tx.Migrator().AddColumn("api_tokens", "expires_at"); err != nil {
				log.Errorf("Failed to add expires_at column: %v", err)
				return err
			}
		}

		if !tx.Migrator().HasColumn("api_tokens", "last_used_at") {
			if err := tx.Migrator().AddColumn("api_tokens", "last_used_at"); err != nil {
				log.Errorf("Failed to add last_used_at column: %v", err)
				return err
			}
		}

		// Drop the old global unique index on name if it exists
		if tx.Migrator().HasIndex("api_tokens", "idx_api_tokens_name") {
			if err := tx.Migrator().DropIndex("api_tokens", "idx_api_tokens_name"); err != nil {
				log.Errorf("Failed to drop global unique index on name: %v", err)
				return err
			}
		}

		// Drop any old unique constraint on name column (GORM might have created it as a constraint)
		if tx.Migrator().HasConstraint("api_tokens", "name") {
			if err := tx.Migrator().DropConstraint("api_tokens", "name"); err != nil {
				log.Errorf("Failed to drop name constraint: %v", err)
				// This might fail on some databases if it doesn't exist, so don't return error
			}
		}

		// Create the new composite unique index on (user_id, name)
		if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_token_name ON api_tokens(user_id, name)").Error; err != nil {
			log.Errorf("Failed to create composite unique index: %v", err)
			return err
		}

		return nil
	})
}

// Register this migration
func init() {
	Register(AddAPITokenExpiryAndCompositeUnique20260808{})
}
