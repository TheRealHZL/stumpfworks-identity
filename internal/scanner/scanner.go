package scanner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Scanner interface {
	Scan(context.Context) (string, error)
}

// Command delegates scanning to an optional platform helper and keeps the core agent portable.
type Command struct{ Path string }

func (c Command) Scan(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, c.Path).Output()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", fmt.Errorf("scanner returned an empty payload")
	}
	return v, nil
}

type Manual struct {
	In  io.Reader
	Out io.Writer
}

func (m Manual) Scan(ctx context.Context) (string, error) {
	fmt.Fprint(m.Out, "Badge payload: ")
	ch := make(chan struct {
		v string
		e error
	}, 1)
	go func() {
		v, e := bufio.NewReader(m.In).ReadString('\n')
		ch <- struct {
			v string
			e error
		}{strings.TrimSpace(v), e}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		return r.v, r.e
	}
}
