package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type lspClient struct {
	cmd      *exec.Cmd
	in       io.WriteCloser
	mu       sync.Mutex
	nextID   int
	open     map[string]int
	pending  map[int]chan json.RawMessage
	diagWait map[string]chan struct{}
	closed   chan struct{}
}

func startLSP(root string) (*lspClient, error) {
	exe, err := selfExecutable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, "lsp", "--root", root)
	cmd.Dir = root
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &lspClient{
		cmd:      cmd,
		in:       in,
		open:     make(map[string]int),
		pending:  make(map[int]chan json.RawMessage),
		diagWait: make(map[string]chan struct{}),
		closed:   make(chan struct{}),
	}
	go c.readLoop(out)
	if _, err := c.request("initialize", map[string]any{
		"processId":    nil,
		"rootUri":      "file://" + filepath.ToSlash(root),
		"capabilities": map[string]any{},
	}); err != nil {
		c.close()
		return nil, err
	}
	c.notify("initialized", map[string]any{})
	return c, nil
}

var selfExecutable = os.Executable

func (c *lspClient) close() {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	_ = c.in.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
}

func (c *lspClient) send(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}

func (c *lspClient) request(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	select {
	case raw := <-ch:
		var resp struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("%s: timeout", method)
	}
}

func (c *lspClient) notify(method string, params any) {
	_ = c.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *lspClient) readLoop(r io.Reader) {
	br := bufio.NewReader(r)
	for {
		headers := map[string]string{}
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if k, v, ok := strings.Cut(line, ":"); ok {
				headers[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
			}
		}
		n, err := strconv.Atoi(headers["content-length"])
		if err != nil || n <= 0 {
			return
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(br, body); err != nil {
			return
		}
		c.dispatch(body)
	}
}

func (c *lspClient) dispatch(body []byte) {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			URI string `json:"uri"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &msg) != nil {
		return
	}
	if msg.Method == "textDocument/publishDiagnostics" {
		c.mu.Lock()
		ch := c.diagWait[msg.Params.URI]
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
		return
	}
	if msg.Method != "" || len(msg.ID) == 0 {
		return
	}
	var id int
	if json.Unmarshal(msg.ID, &id) != nil {
		return
	}
	c.mu.Lock()
	ch := c.pending[id]
	c.mu.Unlock()
	if ch != nil {
		ch <- body
	}
}

func (c *lspClient) openFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	uri := "file://" + filepath.ToSlash(abs)
	c.mu.Lock()
	version := c.open[uri] + 1
	c.open[uri] = version
	wait := make(chan struct{}, 1)
	c.diagWait[uri] = wait
	c.mu.Unlock()
	if version == 1 {
		c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "ink", "version": version, "text": string(data),
		}})
	} else {
		c.notify("textDocument/didChange", map[string]any{
			"textDocument":   map[string]any{"uri": uri, "version": version},
			"contentChanges": []map[string]any{{"text": string(data)}},
		})
	}
	select {
	case <-wait:
	case <-time.After(10 * time.Second):
	}
	return uri, nil
}
