package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

type Store struct {
	pool *pgxpool.Pool
}

type Chunk struct {
	Source  string
	Content string
}

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

type Source struct {
	Name   string `json:"name"`
	Chunks int    `json:"chunks"`
}

func (s *Store) ReplaceSource(ctx context.Context, source string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM company_chunks WHERE source = $1`, source)
	if err != nil {
		return fmt.Errorf("replace source: %w", err)
	}
	return nil
}

func (s *Store) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := s.pool.Query(
		ctx,
		`SELECT source, COUNT(*)
		 FROM company_chunks
		 GROUP BY source
		 ORDER BY source`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var out []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.Name, &src.Chunks); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		out = append(out, src)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sources: %w", err)
	}
	return out, nil
}

func (s *Store) Insert(ctx context.Context, source, content string, embedding []float32) error {
	_, err := s.pool.Exec(
		ctx,
		`INSERT INTO company_chunks (source, content, embedding) VALUES ($1, $2, $3)`,
		source,
		content,
		pgvector.NewVector(embedding),
	)
	if err != nil {
		return fmt.Errorf("insert chunk: %w", err)
	}
	return nil
}

func (s *Store) Search(ctx context.Context, embedding []float32, limit int) ([]Chunk, error) {
	rows, err := s.pool.Query(
		ctx,
		`SELECT source, content
		 FROM company_chunks
		 ORDER BY embedding <=> $1
		 LIMIT $2`,
		pgvector.NewVector(embedding),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search chunks: %w", err)
	}
	defer rows.Close()

	var out []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.Source, &c.Content); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chunks: %w", err)
	}

	return out, nil
}
