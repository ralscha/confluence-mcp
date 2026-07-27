package confluence

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestClient_GetPageAttachments(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/attachments"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("limit"), "10"; got != want {
			t.Errorf("limit = %q, want %q", got, want)
		}
		writeJSON(w, `{"results": [{"id": "att1", "title": "diagram.png", "mediaType": "image/png", "fileSize": 2048}]}`)
	})

	result, err := client.GetPageAttachments(context.Background(), "123", 10)
	if err != nil {
		t.Fatalf("GetPageAttachments failed: %v", err)
	}
	if got, want := len(result.Results), 1; got != want {
		t.Fatalf("attachments = %d, want %d", got, want)
	}
	if got, want := result.Results[0].FileSize, int64(2048); got != want {
		t.Errorf("file size = %d, want %d", got, want)
	}
}

func TestClient_UploadAttachment(t *testing.T) {
	var (
		gotToken    string
		gotFilename string
		gotMIME     string
		gotData     []byte
	)

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/pages/123/attachments"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		gotToken = r.Header.Get("X-Atlassian-Token")

		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parsing content type: %v", err)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("reading multipart part: %v", err)
		}
		gotFilename = part.FileName()
		gotMIME = part.Header.Get("Content-Type")
		if gotData, err = io.ReadAll(part); err != nil {
			t.Fatalf("reading part data: %v", err)
		}

		writeJSON(w, `{"results": [{"id": "att1", "title": "diagram.png"}]}`)
	})

	attachment, err := client.UploadAttachment(context.Background(), "123", "diagram.png", "image/png", []byte("png-bytes"))
	if err != nil {
		t.Fatalf("UploadAttachment failed: %v", err)
	}

	if got, want := attachment.ID, "att1"; got != want {
		t.Errorf("attachment ID = %q, want %q", got, want)
	}
	if got, want := gotToken, "no-check"; got != want {
		t.Errorf("X-Atlassian-Token = %q, want %q", got, want)
	}
	if got, want := gotFilename, "diagram.png"; got != want {
		t.Errorf("filename = %q, want %q", got, want)
	}
	if got, want := gotMIME, "image/png"; got != want {
		t.Errorf("part content type = %q, want %q", got, want)
	}
	if got, want := string(gotData), "png-bytes"; got != want {
		t.Errorf("part data = %q, want %q", got, want)
	}
}

func TestClient_UploadAttachmentNoResults(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"results": []}`)
	})

	if _, err := client.UploadAttachment(context.Background(), "123", "a.txt", "", []byte("x")); err == nil {
		t.Fatal("expected error when the API returns no attachment, got nil")
	}
}

func TestClient_DownloadAttachment(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/wiki/api/v2/attachments/att1/data"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte("file-contents"))
	})

	data, err := client.DownloadAttachment(context.Background(), "att1")
	if err != nil {
		t.Fatalf("DownloadAttachment failed: %v", err)
	}
	if got, want := string(data), "file-contents"; got != want {
		t.Errorf("data = %q, want %q", got, want)
	}
}

func TestClient_DownloadAttachmentError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		writeJSON(w, `{"message": "No permission"}`)
	})

	_, err := client.DownloadAttachment(context.Background(), "att1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "No permission") {
		t.Errorf("error = %q, want it to include the API message", err)
	}
}

func TestClient_DownloadAttachmentTooLargeContentLength(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(maxAttachmentBytes+1))
		_, _ = io.Copy(w, bytes.NewReader(make([]byte, maxAttachmentBytes+1)))
	})

	_, err := client.DownloadAttachment(context.Background(), "att1")
	if err == nil {
		t.Fatal("expected error for oversized attachment, got nil")
	}
	if !strings.Contains(err.Error(), "exceeding") {
		t.Errorf("error = %q, want it to mention the download limit", err)
	}
}

func TestClient_DownloadAttachmentTooLargeUnknownLength(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// No Content-Length: the response is chunked, so the size is only
		// discovered while reading.
		w.WriteHeader(http.StatusOK)
		for written := 0; written <= maxAttachmentBytes; written += 64 << 10 {
			if _, err := w.Write(make([]byte, 64<<10)); err != nil {
				return
			}
		}
	})

	_, err := client.DownloadAttachment(context.Background(), "att1")
	if err == nil {
		t.Fatal("expected error for oversized attachment, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error = %q, want it to mention the size limit", err)
	}
}

func TestClient_DeleteAttachment(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodDelete; got != want {
			t.Errorf("method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/wiki/api/v2/attachments/att1"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeleteAttachment(context.Background(), "att1"); err != nil {
		t.Fatalf("DeleteAttachment failed: %v", err)
	}
}
