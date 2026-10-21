package digestauth

import (
	"fmt"
	stdhttp "net/http"
	"regexp"
	"testing"

	duehttp "github.com/dobyte/due/component/http/v2"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xhash"
	"github.com/gofiber/fiber/v3"
)

func TestDigestURI(t *testing.T) {
	const (
		username = "user"
		password = "password"
		realm    = "test"
	)

	middleware := New(Config{
		Realm: realm,
		Users: map[string]string{
			username: xhash.MD5(username + ":" + realm + ":" + password),
		},
	})

	server := duehttp.NewServer()
	server.Proxy().Router().Get("/resource", middleware, func(ctx duehttp.Context) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	})
	server.Proxy().Router().Get("/other", middleware, func(ctx duehttp.Context) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	req, err := stdhttp.NewRequest(stdhttp.MethodGet, "/resource", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := server.Proxy().App().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	nonce := extractNonce(res.Header.Get("WWW-Authenticate"))
	if nonce == "" {
		t.Fatal("digest challenge nonce is empty")
	}

	authorization := makeAuthorization(username, password, realm, nonce, "/resource")

	req, err = stdhttp.NewRequest(stdhttp.MethodGet, "/other", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(fiber.HeaderAuthorization, authorization)

	res, err = server.Proxy().App().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("mismatched digest URI status: got %d, want %d", res.StatusCode, fiber.StatusBadRequest)
	}

	req, err = stdhttp.NewRequest(stdhttp.MethodGet, "/resource", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(fiber.HeaderAuthorization, authorization)

	res, err = server.Proxy().App().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != fiber.StatusNoContent {
		t.Fatalf("matching digest URI status: got %d, want %d", res.StatusCode, fiber.StatusNoContent)
	}
}

func TestGenerateNonceRandomFailure(t *testing.T) {
	wantErr := errors.New("random source failed")
	da := newDigestAuth()
	da.readRandom = func([]byte) (int, error) {
		return 0, wantErr
	}

	nonce, err := da.generateNonce()
	if !errors.Is(err, wantErr) {
		t.Fatalf("generate nonce error: got %v, want %v", err, wantErr)
	}
	if nonce != "" {
		t.Fatalf("generate nonce on failure: got %q, want empty", nonce)
	}
	if len(da.nonces) != 0 {
		t.Fatalf("cached nonce count on failure: got %d, want 0", len(da.nonces))
	}
}

var _noncePattern = regexp.MustCompile(`nonce="([^"]+)"`)

func extractNonce(challenge string) string {
	matches := _noncePattern.FindStringSubmatch(challenge)
	if len(matches) != 2 {
		return ""
	}

	return matches[1]
}

func makeAuthorization(username, password, realm, nonce, uri string) string {
	const (
		nonceCount  = "00000001"
		clientNonce = "client-nonce"
	)

	ha1 := xhash.MD5(username + ":" + realm + ":" + password)
	ha2 := xhash.MD5(stdhttp.MethodGet + ":" + uri)
	response := xhash.MD5(ha1 + ":" + nonce + ":" + nonceCount + ":" + clientNonce + ":auth:" + ha2)

	return fmt.Sprintf(
		`Digest username="%s", realm="%s", nonce="%s", uri="%s", qop=auth, nc=%s, cnonce="%s", response="%s"`,
		username,
		realm,
		nonce,
		uri,
		nonceCount,
		clientNonce,
		response,
	)
}
