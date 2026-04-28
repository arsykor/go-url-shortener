package pool_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arsykor/go-url-shortener/internal/pool"
	"github.com/arsykor/go-url-shortener/internal/service"
)

func TestPool_GetReturnsObject(t *testing.T) {
	p := pool.New(func() *service.BatchItem { return &service.BatchItem{} })

	item := p.Get()
	assert.NotNil(t, item)
}

func TestPool_PutResetsBeforeReuse(t *testing.T) {
	p := pool.New(func() *service.BatchItem { return &service.BatchItem{} })

	// Borrow, fill, return.
	item := p.Get()
	item.ShortID = "abc123"
	item.OriginalURL = "https://practicum.yandex.ru"
	p.Put(item)

	// The same pointer comes back with zeroed fields.
	reused := p.Get()
	assert.Equal(t, "", reused.ShortID)
	assert.Equal(t, "", reused.OriginalURL)
}
