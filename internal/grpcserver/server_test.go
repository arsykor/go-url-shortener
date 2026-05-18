package grpcserver

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/arsykor/go-url-shortener/internal/middleware"
	"github.com/arsykor/go-url-shortener/internal/repository"
	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/arsykor/go-url-shortener/internal/urlapi"
	pb "github.com/arsykor/go-url-shortener/proto/shortenerv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

func TestServer_ShortenAndList(t *testing.T) {
	logger := zap.NewNop().Sugar()
	repo := repository.NewInMemoryURLRepository()
	svc := service.NewShortenerService(repo, "http://localhost:8080", logger)
	f := &urlapi.Facade{Svc: svc, Audit: nil}

	gs := grpc.NewServer()
	Register(gs, &Server{Facade: f, Logger: logger})

	buf := bufconn.Listen(1024 * 1024)
	go func() { _ = gs.Serve(buf) }()
	t.Cleanup(func() { gs.Stop() })

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return buf.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	client := pb.NewShortenerServiceClient(conn)

	uid := "test-user-1"
	auth := middleware.SignedUserToken(uid)
	shortCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", auth)
	shortReq := (&pb.URLShortenRequest_builder{Url: "https://practicum.yandex.ru"}).Build()
	res, err := client.ShortenURL(shortCtx, shortReq)
	require.NoError(t, err)
	require.NotEmpty(t, res.GetResult())

	id := strings.TrimPrefix(res.GetResult(), "http://localhost:8080/")
	exReq := (&pb.URLExpandRequest_builder{Id: id}).Build()
	ex, err := client.ExpandURL(context.Background(), exReq)
	require.NoError(t, err)
	assert.Equal(t, "https://practicum.yandex.ru", ex.GetResult())

	ctxList := metadata.AppendToOutgoingContext(context.Background(), "authorization", auth)
	list, err := client.ListUserURLs(ctxList, (&pb.ListUserURLsRequest_builder{}).Build())
	require.NoError(t, err)
	urls := list.GetUrl()
	require.Len(t, urls, 1)
	assert.Equal(t, "https://practicum.yandex.ru", urls[0].GetOriginalUrl())
}
