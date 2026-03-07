package memory_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bakdaulet/ais/ais-back/internal/domain/repository"
	"github.com/bakdaulet/ais/ais-back/internal/infrastructure/memory"
	apperrors "github.com/bakdaulet/ais/ais-back/pkg/errors"
)

func makeRepo(id, url string) *repository.Repo {
	return &repository.Repo{
		ID:        id,
		URL:       url,
		Owner:     "owner",
		Name:      "repo",
		Status:    repository.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestRepoRepository_CreateAndGet(t *testing.T) {
	repo := memory.NewRepoRepository()
	ctx := context.Background()

	r := makeRepo("id-1", "https://github.com/owner/repo")
	if err := repo.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "id-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != "id-1" {
		t.Errorf("got ID %q; want id-1", got.ID)
	}
}

func TestRepoRepository_Create_Duplicate(t *testing.T) {
	repo := memory.NewRepoRepository()
	ctx := context.Background()

	r := makeRepo("id-dup", "https://github.com/owner/repo")
	_ = repo.Create(ctx, r)
	err := repo.Create(ctx, r)
	if err == nil {
		t.Fatal("expected error on duplicate create, got nil")
	}
	de, ok := err.(*apperrors.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if de.Code != apperrors.ErrCodeAlreadyExists {
		t.Errorf("code = %q; want ALREADY_EXISTS", de.Code)
	}
}

func TestRepoRepository_GetByID_NotFound(t *testing.T) {
	repo := memory.NewRepoRepository()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	de, ok := err.(*apperrors.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if de.Code != apperrors.ErrCodeNotFound {
		t.Errorf("code = %q; want NOT_FOUND", de.Code)
	}
}

func TestRepoRepository_Update(t *testing.T) {
	repo := memory.NewRepoRepository()
	ctx := context.Background()

	r := makeRepo("id-upd", "https://github.com/owner/repo")
	_ = repo.Create(ctx, r)

	r.Status = repository.StatusReady
	r.FileCount = 42
	if err := repo.Update(ctx, r); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := repo.GetByID(ctx, "id-upd")
	if got.Status != repository.StatusReady {
		t.Errorf("status = %q; want ready", got.Status)
	}
	if got.FileCount != 42 {
		t.Errorf("fileCount = %d; want 42", got.FileCount)
	}
}

func TestRepoRepository_GetByURL(t *testing.T) {
	repo := memory.NewRepoRepository()
	ctx := context.Background()

	url := "https://github.com/owner/unique-repo"
	r := makeRepo("id-url", url)
	_ = repo.Create(ctx, r)

	got, err := repo.GetByURL(ctx, url)
	if err != nil {
		t.Fatalf("GetByURL: %v", err)
	}
	if got.URL != url {
		t.Errorf("URL = %q; want %q", got.URL, url)
	}
}

func TestRepoRepository_List(t *testing.T) {
	repo := memory.NewRepoRepository()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("id-%d", i)
		url := fmt.Sprintf("https://github.com/owner/repo-%d", i)
		_ = repo.Create(ctx, makeRepo(id, url))
		time.Sleep(1 * time.Millisecond) // ensure distinct CreatedAt
	}

	all, total, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d; want 5", total)
	}
	if len(all) != 5 {
		t.Errorf("len = %d; want 5", len(all))
	}

	page, _, _ := repo.List(ctx, 2, 0)
	if len(page) != 2 {
		t.Errorf("paginated len = %d; want 2", len(page))
	}
}

func TestRepoRepository_Delete(t *testing.T) {
	repo := memory.NewRepoRepository()
	ctx := context.Background()

	r := makeRepo("id-del", "https://github.com/owner/repo")
	_ = repo.Create(ctx, r)

	if err := repo.Delete(ctx, "id-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.GetByID(ctx, "id-del")
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}
