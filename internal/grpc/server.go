package grpcserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jyotil-raval/mal-updater/internal/config"
	"github.com/jyotil-raval/mal-updater/internal/mal"
	pb "github.com/jyotil-raval/mal-updater/proto/animepb"
)

type AnimeServer struct {
	pb.UnimplementedAnimeServiceServer
	accessToken string
}

func NewAnimeServer(accessToken string) *AnimeServer {
	return &AnimeServer{accessToken: accessToken}
}

func (s *AnimeServer) GetAnime(ctx context.Context, req *pb.GetAnimeRequest) (*pb.AnimeResponse, error) {
	fields := "id,title,synopsis,media_type,status,num_episodes,start_date,end_date,mean,rank,popularity,rating,genres,studios"
	endpoint := fmt.Sprintf("%s/anime/%s?fields=%s", config.MALAPIBaseURL, req.Id, fields)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.accessToken)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling MAL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAL returned %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return mapToProto(data), nil
}

func (s *AnimeServer) Search(ctx context.Context, req *pb.SearchAnimeRequest) (*pb.SearchAnimeResponse, error) {
	endpoint := fmt.Sprintf("%s/anime?q=%s&limit=20&fields=id,title,media_type,status,mean,genres,num_episodes",
		config.MALAPIBaseURL, req.Q)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.accessToken)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling MAL: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var items []*pb.AnimeResponse
	for _, item := range result.Data {
		if node, ok := item["node"].(map[string]any); ok {
			items = append(items, mapToProto(node))
		}
	}

	return &pb.SearchAnimeResponse{Data: items}, nil
}

func (s *AnimeServer) GetList(ctx context.Context, req *pb.GetListRequest) (*pb.GetListResponse, error) {
	entries, err := mal.GetAnimeList(s.accessToken)
	if err != nil {
		return nil, fmt.Errorf("fetching MAL list: %w", err)
	}

	var items []*pb.AnimeResponse
	for _, e := range entries {
		if req.Status != "" && e.ListStatus.Status != req.Status {
			continue
		}
		if req.MinScore > 0 && int32(e.ListStatus.Score) < req.MinScore {
			continue
		}
		items = append(items, &pb.AnimeResponse{
			Id:          strconv.Itoa(e.Node.ID),
			Title:       e.Node.Title,
			MediaType:   e.Node.MediaType,
			NumEpisodes: int32(e.Node.NumEpisodes),
		})
	}

	return &pb.GetListResponse{
		Total: int32(len(items)),
		Data:  items,
	}, nil
}

func mapToProto(data map[string]any) *pb.AnimeResponse {
	r := &pb.AnimeResponse{}

	if v, ok := data["id"].(float64); ok {
		r.Id = strconv.Itoa(int(v))
	}
	if v, ok := data["title"].(string); ok {
		r.Title = v
	}
	if v, ok := data["synopsis"].(string); ok {
		r.Synopsis = v
	}
	if v, ok := data["media_type"].(string); ok {
		r.MediaType = v
	}
	if v, ok := data["status"].(string); ok {
		r.Status = v
	}
	if v, ok := data["num_episodes"].(float64); ok {
		r.NumEpisodes = int32(v)
	}
	if v, ok := data["start_date"].(string); ok {
		r.StartDate = v
	}
	if v, ok := data["end_date"].(string); ok {
		r.EndDate = v
	}
	if v, ok := data["mean"].(float64); ok {
		r.MeanScore = v
	}
	if v, ok := data["rank"].(float64); ok {
		r.Rank = int32(v)
	}
	if v, ok := data["popularity"].(float64); ok {
		r.Popularity = int32(v)
	}
	if v, ok := data["rating"].(string); ok {
		r.Rating = v
	}
	if genres, ok := data["genres"].([]any); ok {
		for _, g := range genres {
			if gMap, ok := g.(map[string]any); ok {
				genre := &pb.Genre{}
				if id, ok := gMap["id"].(float64); ok {
					genre.Id = int32(id)
				}
				if name, ok := gMap["name"].(string); ok {
					genre.Name = name
				}
				r.Genres = append(r.Genres, genre)
			}
		}
	}
	if studios, ok := data["studios"].([]any); ok {
		for _, s := range studios {
			if sMap, ok := s.(map[string]any); ok {
				studio := &pb.Studio{}
				if id, ok := sMap["id"].(float64); ok {
					studio.Id = int32(id)
				}
				if name, ok := sMap["name"].(string); ok {
					studio.Name = name
				}
				r.Studios = append(r.Studios, studio)
			}
		}
	}

	return r
}
