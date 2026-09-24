package client

import (
	"errors"
	"fmt"
	"strings"

	"newreleases.io/newreleases"
)

// IsNotFound reports whether err means the requested object does not exist.
func IsNotFound(err error) bool { return errors.Is(err, newreleases.ErrNotFound) }

// Detail formats an API error as diagnostic detail text, expanding the list of
// reasons carried by a BadRequestError.
func Detail(op string, err error) string {
	var bre *newreleases.BadRequestError
	if errors.As(err, &bre) {
		return fmt.Sprintf("%s: %s", op, strings.Join(bre.Errors(), "; "))
	}
	return fmt.Sprintf("%s: %s", op, err)
}
