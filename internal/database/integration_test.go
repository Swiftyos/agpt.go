//go:build integration

// Package database contains integration tests for database operations.
// These tests require a PostgreSQL database connection via DATABASE_URL.
//
// Run with: go test -v -tags=integration ./internal/database/...
//
// These tests are designed to run in the nightly E2E workflow,
// not during regular CI to avoid requiring database setup.
package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/agpt-go/chatbot-api/internal/config"
)

// skipIfNoDatabase skips the test if DATABASE_URL is not set
func skipIfNoDatabase(t *testing.T) {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set, skipping database integration test")
	}
}

// getDatabaseConfig returns a DatabaseConfig parsed from DATABASE_URL
func getDatabaseConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()
	cfg, err := config.ParseDatabaseURL(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("Failed to parse DATABASE_URL: %v", err)
	}
	return cfg
}

// TestIntegration_DatabaseConnection tests basic database connectivity
func TestIntegration_DatabaseConnection(t *testing.T) {
	skipIfNoDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, getDatabaseConfig(t))
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Test simple query
	var result int
	err = pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}

	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}

// TestIntegration_ConnectionPooling tests connection pool behavior
func TestIntegration_ConnectionPooling(t *testing.T) {
	skipIfNoDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, getDatabaseConfig(t))
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Execute multiple concurrent queries
	done := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			var result int
			err := pool.QueryRow(ctx, "SELECT $1::int", id).Scan(&result)
			if err != nil {
				done <- err
				return
			}
			if result != id {
				done <- err
			}
			done <- nil
		}(i)
	}

	// Wait for all queries to complete
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent query failed: %v", err)
		}
	}
}

// TestIntegration_TransactionSupport tests transaction handling
func TestIntegration_TransactionSupport(t *testing.T) {
	skipIfNoDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, getDatabaseConfig(t))
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Start a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Execute query in transaction
	var result int
	err = tx.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("Failed to execute query in transaction: %v", err)
	}

	// Rollback (cleanup - we don't want to persist anything in test)
	err = tx.Rollback(ctx)
	if err != nil {
		t.Errorf("Failed to rollback: %v", err)
	}
}

// TestIntegration_ContextCancellation tests that context cancellation works
func TestIntegration_ContextCancellation(t *testing.T) {
	skipIfNoDatabase(t)

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())

	pool, err := NewPool(ctx, getDatabaseConfig(t))
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Cancel after starting a query
	cancel()

	// This should fail due to cancelled context
	var result int
	err = pool.QueryRow(ctx, "SELECT pg_sleep(10)").Scan(&result)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

// TestIntegration_PreparedStatements tests prepared statement caching
func TestIntegration_PreparedStatements(t *testing.T) {
	skipIfNoDatabase(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, getDatabaseConfig(t))
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Execute the same query multiple times (should use prepared statement cache)
	for i := 0; i < 5; i++ {
		var result int
		err := pool.QueryRow(ctx, "SELECT $1::int * 2", i).Scan(&result)
		if err != nil {
			t.Fatalf("Query %d failed: %v", i, err)
		}
		if result != i*2 {
			t.Errorf("Query %d: expected %d, got %d", i, i*2, result)
		}
	}
}
