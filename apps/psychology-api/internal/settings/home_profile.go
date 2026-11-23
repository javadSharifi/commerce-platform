package settings

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidHomeProfile = errors.New("settings: invalid home profile")
	ErrInvalidPortrait    = errors.New("settings: portrait must be a verified public image of this clinic")
)

type HomeProfile struct {
	Introduction    string     `json:"introduction,omitempty"`
	Specialties     []string   `json:"specialties,omitempty"`
	Credentials     []string   `json:"credentials,omitempty"`
	OfficeHours     string     `json:"office_hours,omitempty"`
	PortraitMediaID *uuid.UUID `json:"portrait_media_id,omitempty"`
}

func encodeHomeProfile(home *HomeProfile) (pgtype.Text, pgtype.UUID, error) {
	if home == nil {
		return pgtype.Text{}, pgtype.UUID{}, nil
	}
	clean := HomeProfile{
		Introduction: strings.TrimSpace(home.Introduction),
		OfficeHours:  strings.TrimSpace(home.OfficeHours),
	}
	if utf8.RuneCountInString(clean.Introduction) > 2000 || utf8.RuneCountInString(clean.OfficeHours) > 240 {
		return pgtype.Text{}, pgtype.UUID{}, ErrInvalidHomeProfile
	}
	var err error
	clean.Specialties, err = cleanProfileItems(home.Specialties)
	if err != nil {
		return pgtype.Text{}, pgtype.UUID{}, err
	}
	clean.Credentials, err = cleanProfileItems(home.Credentials)
	if err != nil {
		return pgtype.Text{}, pgtype.UUID{}, err
	}
	var portrait pgtype.UUID
	if home.PortraitMediaID != nil {
		if *home.PortraitMediaID == uuid.Nil {
			return pgtype.Text{}, pgtype.UUID{}, ErrInvalidPortrait
		}
		portrait = toPgUUID(*home.PortraitMediaID)
	}
	data, err := json.Marshal(clean)
	return pgtype.Text{String: string(data), Valid: err == nil}, portrait, err
}

func cleanProfileItems(items []string) ([]string, error) {
	if len(items) > 8 {
		return nil, ErrInvalidHomeProfile
	}
	var clean []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if utf8.RuneCountInString(item) > 120 {
			return nil, ErrInvalidHomeProfile
		}
		if item != "" {
			clean = append(clean, item)
		}
	}
	return clean, nil
}

func decodeHomeProfile(raw string, portrait pgtype.UUID) (*HomeProfile, error) {
	var home HomeProfile
	if raw != "" && raw != "{}" {
		if err := json.Unmarshal([]byte(raw), &home); err != nil {
			return nil, err
		}
	}
	if portrait.Valid {
		id := fromPgUUID(portrait)
		home.PortraitMediaID = &id
	}
	if home.Introduction == "" && home.OfficeHours == "" && len(home.Specialties) == 0 &&
		len(home.Credentials) == 0 && home.PortraitMediaID == nil {
		return nil, nil
	}
	return &home, nil
}
