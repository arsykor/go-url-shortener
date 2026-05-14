// Package grpcserver implements the gRPC API for the URL shortener.
package grpcserver

import (
	"context"
	"errors"
	"strings"

	"github.com/arsykor/go-url-shortener/internal/middleware"
	"github.com/arsykor/go-url-shortener/internal/service"
	"github.com/arsykor/go-url-shortener/internal/urlapi"
	pb "github.com/arsykor/go-url-shortener/proto/shortenerv1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server implements shortenerv1.ShortenerServiceServer.
type Server struct {
	pb.UnimplementedShortenerServiceServer
	Facade *urlapi.Facade
}

func concatAuthorization(md metadata.MD) string {
	parts := md.Get("authorization")
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}

// ShortenURL mirrors POST /api/shorten.
func (s *Server) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	userID := middleware.UserIDFromAuthorizationHeader(concatAuthorization(md))
	if userID == "" {
		userID = uuid.New().String()
		if err := grpc.SetHeader(ctx, metadata.Pairs("authorization", middleware.SignedUserToken(userID))); err != nil {
			return nil, status.Errorf(codes.Internal, "set header: %v", err)
		}
	}
	res, _, err := s.Facade.Shorten(ctx, userID, req.GetUrl())
	if errors.Is(err, urlapi.ErrEmptyURL) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &pb.URLShortenResponse{Result: res}, nil
}

// ExpandURL mirrors GET /{id} (returns the target URL in the message).
func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	orig, err := s.Facade.Expand(ctx, req.GetId())
	if errors.Is(err, service.ErrURLNotFound) {
		return nil, status.Error(codes.InvalidArgument, "unknown id")
	}
	if errors.Is(err, service.ErrURLDeleted) {
		return nil, status.Error(codes.FailedPrecondition, "url deleted")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	if s.Facade.Audit != nil {
		md, _ := metadata.FromIncomingContext(ctx)
		uid := middleware.UserIDFromAuthorizationHeader(concatAuthorization(md))
		s.Facade.Audit.Notify("follow", uid, orig)
	}
	return &pb.URLExpandResponse{Result: orig}, nil
}

// ListUserURLs mirrors GET /api/user/urls.
func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	userID := middleware.UserIDFromAuthorizationHeader(concatAuthorization(md))
	if userID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization")
	}
	list, err := s.Facade.ListUserURLsDisplay(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	resp := &pb.UserURLsResponse{}
	for _, u := range list {
		resp.Url = append(resp.Url, &pb.URLData{ShortUrl: u.ShortURL, OriginalUrl: u.OriginalURL})
	}
	return resp, nil
}

// Register registers the service implementation on srv.
func Register(srv *grpc.Server, impl *Server) {
	pb.RegisterShortenerServiceServer(srv, impl)
}
