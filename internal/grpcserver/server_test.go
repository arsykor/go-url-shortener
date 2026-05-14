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
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestServer_ShortenAndList(t *testing.T) {
	logger := zap.NewNop().Sugar()
	repo := repository.NewInMemoryURLRepository()
	svc := service.NewShortenerService(repo, "http://localhost:8080", logger)
	f := &urlapi.Facade{Svc: svc, Audit: nil}

	gs := grpc.NewServer()
	Register(gs, &Server{Facade: f})

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
	res, err := client.ShortenURL(shortCtx, &pb.URLShortenRequest{Url: "https://practicum.yandex.ru"})
	require.NoError(t, err)
	require.NotEmpty(t, res.GetResult())

	id := strings.TrimPrefix(res.GetResult(), "http://localhost:8080/")
	ex, err := client.ExpandURL(context.Background(), &pb.URLExpandRequest{Id: id})
	require.NoError(t, err)
	assert.Equal(t, "https://practicum.yandex.ru", ex.GetResult())

	ctxList := metadata.AppendToOutgoingContext(context.Background(), "authorization", auth)
	list, err := client.ListUserURLs(ctxList, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, list.GetUrl(), 1)
	assert.Equal(t, "https://practicum.yandex.ru", list.Url[0].OriginalUrl)
}
