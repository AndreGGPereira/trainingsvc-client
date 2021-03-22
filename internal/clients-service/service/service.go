package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/pedidopago/trainingsvc-clients/protos/pb"
	"github.com/pedidopago/trainingsvc-clients/utils"
	"google.golang.org/grpc"
)

type Config struct {
	DBCS string
}

func New(ctx context.Context, sv *grpc.Server, config Config) error {

	svc := &Service{}

	// database connection
	db, err := sqlx.Open("mysql", config.DBCS)
	if err != nil {
		return err
	}
	svc.db = db

	go svc.cleanup(ctx) // executa antes de fechar o app

	pb.RegisterClientsServiceServer(sv, svc)

	return nil
}

type Service struct {
	db *sqlx.DB
}

func (s *Service) cleanup(ctx context.Context) {
	<-ctx.Done()
	s.db.Close()
}

var _ pb.ClientsServiceServer = (*Service)(nil) // compile time check if we support the public proto interface

// NewClient creates a new client on the database
func (s *Service) NewClient(ctx context.Context, req *pb.NewClientRequest) (*pb.NewClientResponse, error) {
	//FIXME: this method

	id := utils.SecureID().String()

	cols := make([]string, 0)
	vals := make([]interface{}, 0)

	cols, vals = append(cols, "id"), append(vals, id)

	//FIXME: adicionar name
	if req.Name != "" {
		cols, vals = append(cols, "name"), append(vals, req.Name)
	}
	if req.Birthday != 0 {
		cols, vals = append(cols, "birthday"), append(vals, time.Unix(0, req.Birthday))
	}
	//FIXME: adicionar score
	if req.Score == 0 {
		cols, vals = append(cols, "score"), append(vals, req.Score)
	}
	q, args, err := sq.Insert("clients").Columns(cols...).Values(vals...).ToSql()

	if err != nil {
		return nil, err
	}

	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {

		return nil, err
	}

	// //FIXME: executar query com s.db.ExecCtx...

	return &pb.NewClientResponse{
		Id: id,
	}, nil
}

func (s *Service) QueryClients(ctx context.Context, req *pb.QueryClientsRequest) (*pb.QueryClientsResponse, error) {

	rq := sq.Select("id").From("clients")
	if req.Id != nil {
		rq = rq.Where("id", req.Id.Value)
	}
	if req.Name != nil {
		rq = rq.Where("name LIKE ?", req.Name.Value)
	}
	if req.Birthday != nil {
		rq = req.Birthday.Where("birthday", rq)
	}
	if req.Score != nil {
		rq = req.Score.Where("score", rq)
	}
	if req.CreatedAt != nil {
		rq = req.CreatedAt.Where("created_at", rq)
	}
	rq = rq.OrderBy("score DESC")

	//FIXME: adicionar created_at
	//FIXME: ordenar por score! (DESC)

	q, args, err := rq.ToSql()

	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	if err := s.db.SelectContext(ctx, &ids, q, args...); err != nil {
		return nil, err
	}

	return &pb.QueryClientsResponse{
		Ids: ids,
	}, nil
}

func (s *Service) GetClients(ctx context.Context, req *pb.GetClientsRequest) (*pb.GetClientsResponse, error) {

	ifids := make([]interface{}, 0, len(req.Ids))
	for _, v := range req.Ids {
		ifids = append(ifids, v)
	}

	q, args, err := sq.Select("id", "name", "birthday", "score", "created_at").From("`clients`").
		Where(fmt.Sprintf("id IN (%s)", sq.Placeholders(len(ifids))), ifids...).ToSql()
	if err != nil {
		return nil, err
	}
	rawclients := []struct {
		ID        string        `db:"id"`
		Name      string        `db:"name"`
		Birthday  sql.NullTime  `db:"birthday"`
		Score     sql.NullInt64 `db:"score"`
		CreatedAt sql.NullTime  `db:"created_at"`
	}{}
	if err := s.db.SelectContext(ctx, &rawclients, q, args...); err != nil {
		return nil, err
	}
	resp := &pb.GetClientsResponse{
		Clients: make([]*pb.Client, 0, len(rawclients)),
	}
	for _, v := range rawclients {
		resp.Clients = append(resp.Clients, &pb.Client{
			Id:        v.ID,
			Name:      v.Name,
			Birthday:  v.Birthday.Time.UnixNano(),
			Score:     v.Score.Int64,
			CreatedAt: v.CreatedAt.Time.UnixNano(),
			//FIXME: adicionar created_at
		})
	}

	return resp, nil
}

//FIXME: implementar NewMatch

func (s *Service) NewMatch(ctx context.Context, req *pb.NewMatchRequest) (*pb.NewMatchResponse, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}

	defer tx.Commit()

	var newMatchId int64
	if insert, err := tx.Exec("INSERT INTO client_matches (client_id, score) VALUES (?, ?)", req.ClientId, req.Score); err != nil {
		_ = tx.Rollback()
		return nil, err

	} else {
		newMatchId, _ = insert.LastInsertId()

	}

	if _, err := tx.Exec("UPDATE clients SET score = score + ? WHERE id = ?", req.Score, req.ClientId); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	//FIXME: tx -> UPDATE clients SET score = score + ? WHERE id = ?
	//tx.Rollback()
	//TODO: remover esta linha
	return &pb.NewMatchResponse{Id: newMatchId}, err
}

func (s *Service) DeleteClient(ctx context.Context, req *pb.DeleteClientRequest) (*pb.DeleteClientResponse, error) {

	rq := sq.Delete("clients").Where(sq.Eq{"id": req.Id})
	q, args, err := rq.ToSql()

	if err != nil {
		return nil, err
	}

	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return nil, err
	}

	//FIXME: implementar DeleteClient()
	return &pb.DeleteClientResponse{}, nil
}

func (s *Service) DeleteAllClients(ctx context.Context, req *pb.DeleteAllClientsRequest) (*pb.DeleteAllClientsResponse, error) {

	if _, err := s.db.ExecContext(ctx, "DELETE FROM clients"); err != nil {
		return nil, err
	}

	return &pb.DeleteAllClientsResponse{}, nil
}

//FIXME: implementar Sort()
func (s *Service) Sort(ctx context.Context, req *pb.SortRequest) (*pb.SortResponse, error) {

	strings := make(map[string]bool)
	sortList := []string{}
	sort.Strings(sortList)

	if !req.RemoveDuplicates {
		return &pb.SortResponse{Items: sortList}, nil
	}

	for _, entry := range req.Items {
		if _, value := strings[entry]; !value {
			strings[entry] = true
			sortList = append(sortList, entry)
		}
	}

	return &pb.SortResponse{Items: sortList}, nil
}
