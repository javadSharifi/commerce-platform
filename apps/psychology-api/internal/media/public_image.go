package media

import (
	"context"
	"errors"

	"psychology/api/internal/media/mediadb"
	"psychology/api/internal/platform/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func readPublicAsset(ctx context.Context, q MediaQuerier, tenantID, assetID uuid.UUID) (mediadb.MediaAsset, error) {
	asset, err := q.GetMediaAssetByID(ctx, mediadb.GetMediaAssetByIDParams{
		TenantID: pgtype.UUID{Bytes: tenantID, Valid: true},
		ID:       pgtype.UUID{Bytes: assetID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return mediadb.MediaAsset{}, ErrMediaNotFound
	}
	if err != nil {
		return mediadb.MediaAsset{}, err
	}
	if asset.AccessClass != "public" {
		return mediadb.MediaAsset{}, ErrMediaNotPublic
	}
	if asset.ValidationStatus != "verified" {
		return mediadb.MediaAsset{}, ErrMediaNotVerified
	}
	return asset, nil
}

func (s *Service) publicAsset(ctx context.Context, tenantID, assetID uuid.UUID) (mediadb.MediaAsset, error) {
	if s.publicStore == nil {
		return readPublicAsset(ctx, s.db, tenantID, assetID)
	}
	return db.ReadTenant(ctx, s.publicStore, tenantID, nil, func(tx pgx.Tx) *mediadb.Queries { return mediadb.New(tx) },
		func(q *mediadb.Queries) (mediadb.MediaAsset, error) {
			asset, err := readPublicAsset(ctx, q, tenantID, assetID)
			if err != nil {
				return mediadb.MediaAsset{}, err
			}
			linked, err := q.HasPublicMediaReference(ctx, mediadb.HasPublicMediaReferenceParams{
				TenantID: asset.TenantID, MediaID: asset.ID,
			})
			if err != nil {
				return mediadb.MediaAsset{}, err
			}
			if !linked.Bool {
				return mediadb.MediaAsset{}, ErrMediaNotFound
			}
			return asset, nil
		},
	)
}

func validatePublicImage(asset mediadb.MediaAsset) error {
	switch asset.MimeType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return ErrInvalidMimeType
	}
	if asset.SizeBytes <= 0 || asset.SizeBytes > MaxPublicImageBytes {
		return ErrFileSizeExceeded
	}
	return nil
}

// ValidatePublicImage checks the reference inside the caller's tenant transaction.
func ValidatePublicImage(ctx context.Context, tx pgx.Tx, tenantID, assetID uuid.UUID) error {
	asset, err := readPublicAsset(ctx, mediadb.New(tx), tenantID, assetID)
	if err != nil {
		return err
	}
	return validatePublicImage(asset)
}

func (s *Service) GetPublicImageURL(ctx context.Context, tenantID, assetID uuid.UUID) (string, error) {
	asset, err := s.publicAsset(ctx, tenantID, assetID)
	if err != nil {
		return "", err
	}
	if err := validatePublicImage(asset); err != nil {
		return "", err
	}
	return s.storage.PresignGetURL(ctx, asset.StorageKey, StreamURLTTL)
}
