package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
	"go.uber.org/zap"
)

const baseURL = "http://localhost:8080"

func newService() *service.ShortenerService {
	repo := repository.NewInMemoryURLRepository()
	logger := zap.NewNop().Sugar()
	return service.NewShortenerService(repo, baseURL, logger)
}

// Single URL shortening round-trip.
func BenchmarkShortenURL(b *testing.B) {
	svc := newService()
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		url := fmt.Sprintf("https://practicum.yandex.ru/learn/go-advanced/courses/%d", i)
		svc.ShortenURL(ctx, url, "user1")
	}
}

// Retrieval by short ID.
func BenchmarkGetOriginalURL(b *testing.B) {
	svc := newService()
	ctx := context.Background()

	shortIDs := make([]string, 1000)
	for i := range shortIDs {
		full, _, _ := svc.ShortenURL(ctx, fmt.Sprintf("https://practicum.yandex.ru/learn/go-advanced/courses/%d", i), "user1")
		shortIDs[i] = full[len(baseURL)+1:]
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GetOriginalURL(ctx, shortIDs[i%1000])
	}
}

// Batch shortening (100 URLs per call).
func BenchmarkShortenURLBatch(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	urls := make([]string, 100)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://practicum.yandex.ru/learn/go-advanced/courses/%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Fresh repo each run so there are no conflicts.
		repo := repository.NewInMemoryURLRepository()
		svc := service.NewShortenerService(repo, baseURL, logger)
		b.StartTimer()

		svc.ShortenURLBatch(ctx, urls, "user1")
	}
}

// BenchmarkGetURLsByUser measures fetching all URLs for a user.
func BenchmarkGetURLsByUser(b *testing.B) {
	svc := newService()
	ctx := context.Background()

	for i := 0; i < 500; i++ {
		svc.ShortenURL(ctx, fmt.Sprintf("https://practicum.yandex.ru/learn/go-advanced/courses/%d", i), "user1")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GetURLsByUser(ctx, "user1")
	}
}
