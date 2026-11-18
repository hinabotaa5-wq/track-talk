package httpx

import (
	"os"
	"strings"
	"testing"
)

func TestDocumentConsentVersionsMatchWeb(t *testing.T) {
	src, err := os.ReadFile("../../../web/lib/document-consent.ts")
	if err != nil {
		t.Fatal(err)
	}
	for document, version := range currentDocumentVersions() {
		if !strings.Contains(string(src), document+`: "`+version+`"`) {
			t.Fatalf("Web version differs for %s", document)
		}
	}
}
