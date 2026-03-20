// Package database provides GORM record structs for API credential-related tables.
package database

import (
	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// ApiCredentialRecord is the GORM record struct for the api_credentials table.
// It embeds GormModelUUID (ID, CreatedAt, UpdatedAt, DeletedAt) and the domain ApiCredentialBase struct.
// All GORM operations (Create, Find, Updates, AutoMigrate) use this type.
type ApiCredentialRecord struct {
	gormmodel.GormModelUUID
	models.ApiCredentialBase
}

// TableName implements the GORM TableName interface.
func (ApiCredentialRecord) TableName() string {
	return "api_credentials"
}

// ToApiCredential converts an ApiCredentialRecord to a domain ApiCredential with all fields populated.
func (r *ApiCredentialRecord) ToApiCredential() *models.ApiCredential {
	return &models.ApiCredential{
		ApiCredentialBase: models.ApiCredentialBase{
			OwnerType:            r.ApiCredentialBase.OwnerType,
			OwnerID:              r.ApiCredentialBase.OwnerID,
			Provider:             r.ApiCredentialBase.Provider,
			CredentialType:       r.ApiCredentialBase.CredentialType,
			CredentialName:       r.ApiCredentialBase.CredentialName,
			CredentialCiphertext: r.ApiCredentialBase.CredentialCiphertext,
			CredentialKID:        r.ApiCredentialBase.CredentialKID,
			IsActive:             r.ApiCredentialBase.IsActive,
			ExpiresAt:            r.ApiCredentialBase.ExpiresAt,
			UpdatedBy:            r.ApiCredentialBase.UpdatedBy,
		},
		ID:        r.GormModelUUID.ID,
		CreatedAt: r.GormModelUUID.CreatedAt,
		UpdatedAt: r.GormModelUUID.UpdatedAt,
	}
}
