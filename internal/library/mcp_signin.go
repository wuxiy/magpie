package library

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/mcpauth"
)

// A remote server that asks for an OAuth sign-in is signed in to once, in
// magpie, and every agent given it reaches it through the gateway's
// /mcp/<name>, which adds magpie's token (#615): the agents are given that
// address in place of the server's. A server magpie isn't signed in to is
// given as it is, and the agent signs in itself as before.

func init() {
	mcpauth.Changed = func(string) { _, _ = Sync() }
}

// through is the server as an agent that reaches the gateway at base is
// given it. Only a streamable HTTP server is relayed: an SSE one's stream
// names the address to post to, which would have to be rewritten.
func through(s *Server, base string) *Server {
	if s.Transport != "http" || !mcpauth.SignedIn(s.Name, s.URL) {
		return s
	}
	c := *s
	c.URL = strings.TrimRight(base, "/") + "/mcp/" + url.PathEscape(s.Name)
	c.Headers = nil
	for k, v := range s.Headers {
		// magpie's token goes in its place
		if strings.EqualFold(k, "Authorization") {
			continue
		}
		if c.Headers == nil {
			c.Headers = map[string]string{}
		}
		c.Headers[k] = v
	}
	return &c
}

// gatewayOf is the gateway's address as the target's agent reaches it.
func gatewayOf(t *Target) string {
	if t.Agent != nil && t.Agent.Gateway != nil {
		return t.Agent.Gateway()
	}
	return gateway.URL()
}

var errNotHTTP = errors.New("magpie signs in only to a server reached over streamable HTTP")

// SignInServer starts signing magpie in to the library's server called
// name: the State's URL is the page to open in a browser.
func SignInServer(ctx context.Context, name string) (mcpauth.State, error) {
	mu.Lock()
	l, err := load()
	mu.Unlock()
	if err != nil {
		return mcpauth.State{}, err
	}
	s := l.server(name)
	if s == nil {
		return mcpauth.State{}, fmt.Errorf("no server called %s", name)
	}
	if s.Transport != "http" {
		return mcpauth.State{}, errNotHTTP
	}
	return mcpauth.Start(ctx, s.Name, s.URL)
}

// SignOutServer forgets magpie's sign-in to the server called name, and
// gives the agents the server's own address again.
func SignOutServer(name string) error { return mcpauth.SignOut(name) }
