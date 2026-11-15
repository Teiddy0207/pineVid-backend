package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAdminServiceRequiresHeader(t *testing.T) {
	for _, tc := range []struct {
		name, secret, header, path string
		want                       int
	}{
		{"missing", "service-secret", "", "/admin", 401},
		{"wrong", "service-secret", "wrong", "/admin", 401},
		{"query rejected", "service-secret", "", "/admin?secret=service-secret", 401},
		{"valid", "service-secret", "service-secret", "/admin", 200},
		{"empty configuration", "", "", "/admin", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fiber.New()
			f.Get("/admin", RequireInternalHeader(tc.secret), func(c *fiber.Ctx) error { return c.SendStatus(200) })
			req := httptest.NewRequest("GET", tc.path, nil)
			req.Header.Set(InternalTokenHeader, tc.header)
			req.Header.Set("Authorization", "Bearer user-jwt")
			res, err := f.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.want {
				t.Fatal(res.StatusCode)
			}
		})
	}
}
