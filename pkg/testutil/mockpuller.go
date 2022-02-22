package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
)

type mockPuller struct {
	nextError error
	nextData  []byte
}

func NewMockPuller() *mockPuller {
	return &mockPuller{
		nextError: fmt.Errorf("no mock data provided"),
	}
}

func (p *mockPuller) Pull(ctx context.Context, ref string) ([]byte, error) {
	if p.nextError != nil {
		return nil, p.nextError
	}

	return p.nextData, nil
}

func (p *mockPuller) WithError(err error) *mockPuller {
	p.nextError = err
	return p
}

func (p *mockPuller) WithData(data []byte) *mockPuller {
	p.nextData = data
	p.nextError = nil
	return p
}

func (p *mockPuller) WithFileData(t *testing.T, filename string) *mockPuller {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("loading test file: %s", err)
		t.FailNow()
		return p
	}

	return p.WithData(data)
}
