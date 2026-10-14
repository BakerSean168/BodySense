package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type episodeReaderStub struct {
	owner       uuid.UUID
	cursor      int64
	episode     uuid.UUID
	listCalls   int
	detailCalls int
	err         error
}

func (s *episodeReaderStub) ListEpisodes(_ context.Context, owner uuid.UUID, cursor int64) (*model.EpisodePage, error) {
	s.owner = owner
	s.cursor = cursor
	s.listCalls++
	if s.err != nil {
		return nil, s.err
	}
	return &model.EpisodePage{UserID: owner, Episodes: []model.EpisodeRecord{}}, nil
}
func (s *episodeReaderStub) GetEpisode(_ context.Context, owner, id uuid.UUID) (*model.EpisodeRecord, error) {
	s.owner = owner
	s.episode = id
	s.detailCalls++
	if s.err != nil {
		return nil, s.err
	}
	return &model.EpisodeRecord{ID: id, UserID: owner}, nil
}

func (s *episodeReaderStub) ListEpisodeRelations(_ context.Context, owner, target uuid.UUID, cursor int64) (*model.EpisodeRelationPage, error) {
	s.owner = owner
	s.episode = target
	s.cursor = cursor
	if s.err != nil {
		return nil, s.err
	}
	return &model.EpisodeRelationPage{UserID: owner, EpisodeID: target, Relations: []model.EpisodeRelationReference{}}, nil
}

func TestEpisodeReadServiceValidatesAuthenticationAndCursors(t *testing.T) {
	stub := &episodeReaderStub{}
	svc := NewEpisodeReadService(stub)
	ctx := context.Background()
	owner := uuid.New()
	target := uuid.New()
	for _, tc := range []struct {
		name   string
		owner  uuid.UUID
		cursor int64
	}{
		{"missing_owner", uuid.Nil, 0},
		{"negative_cursor", owner, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, err := svc.ListEpisodes(ctx, tc.owner, tc.cursor)
			if !errors.Is(err, model.ErrInvalidEpisodeEvidence) || page != nil {
				t.Fatalf("untrusted list accepted %#v %v", page, err)
			}
		})
	}
	if _, err := svc.GetEpisode(ctx, owner, uuid.Nil); !errors.Is(err, model.ErrInvalidEpisodeEvidence) {
		t.Fatalf("empty target accepted: %v", err)
	}
	if _, err := svc.GetEpisode(ctx, uuid.Nil, target); !errors.Is(err, model.ErrInvalidEpisodeEvidence) {
		t.Fatalf("empty owner accepted: %v", err)
	}
	if stub.listCalls != 0 || stub.detailCalls != 0 {
		t.Fatal("invalid command reached repository")
	}

	page, err := svc.ListEpisodes(ctx, owner, 3)
	if err != nil || page.UserID != owner || stub.owner != owner || stub.cursor != 3 || stub.listCalls != 1 {
		t.Fatalf("list=%#v err=%v stub=%#v", page, err, stub)
	}
	detail, err := svc.GetEpisode(ctx, owner, target)
	if err != nil || detail.ID != target || stub.episode != target || stub.detailCalls != 1 {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}

	sentinel := errors.New("isolated read failed")
	stub.err = sentinel
	if _, err := svc.ListEpisodes(ctx, owner, 0); !errors.Is(err, sentinel) {
		t.Fatalf("repository failure masked as empty page: %v", err)
	}
}
