package evidence

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

const invoiceMaxBytes int64 = 20 << 20

var invoiceExtensions = map[string]bool{
	".pdf": true, ".ofd": true, ".xml": true, ".jpg": true, ".jpeg": true, ".png": true,
}

func validateInvoiceCreate(input CreateUploadInput) error {
	if input.ClientContext.Purpose != "invoice" {
		return nil
	}
	extension := strings.ToLower(path.Ext(input.FileName))
	if !invoiceExtensions[extension] || input.SizeBytes > invoiceMaxBytes {
		return ErrInvalid
	}
	return nil
}

func validateInvoiceBytes(fileName string, content []byte) (string, error) {
	if int64(len(content)) > invoiceMaxBytes || len(content) == 0 {
		return "", fmt.Errorf("invoice file exceeds the bounded size")
	}
	extension := strings.ToLower(path.Ext(fileName))
	switch extension {
	case ".pdf":
		if !bytes.HasPrefix(content, []byte("%PDF-")) || bytes.Contains(content, []byte("/JavaScript")) || bytes.Contains(content, []byte("/Launch")) || bytes.Contains(content, []byte("/EmbeddedFile")) {
			return "", fmt.Errorf("unsafe or malformed PDF")
		}
		return "application/pdf", nil
	case ".jpg", ".jpeg":
		if len(content) < 4 || content[0] != 0xff || content[1] != 0xd8 || content[len(content)-2] != 0xff || content[len(content)-1] != 0xd9 {
			return "", fmt.Errorf("malformed JPEG")
		}
		return "image/jpeg", nil
	case ".png":
		if !bytes.HasPrefix(content, []byte("\x89PNG\r\n\x1a\n")) {
			return "", fmt.Errorf("malformed PNG")
		}
		return "image/png", nil
	case ".xml":
		upper := bytes.ToUpper(content)
		if bytes.Contains(upper, []byte("<!DOCTYPE")) || bytes.Contains(upper, []byte("<!ENTITY")) {
			return "", fmt.Errorf("XML external declarations are forbidden")
		}
		decoder := xml.NewDecoder(bytes.NewReader(content))
		for {
			if _, err := decoder.Token(); err != nil {
				if err == io.EOF {
					break
				}
				return "", fmt.Errorf("malformed XML")
			}
		}
		return "application/xml", nil
	case ".ofd":
		reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil || len(reader.File) == 0 || len(reader.File) > 1000 {
			return "", fmt.Errorf("malformed OFD")
		}
		foundManifest := false
		var expanded uint64
		for _, entry := range reader.File {
			cleaned := path.Clean(strings.ReplaceAll(entry.Name, "\\", "/"))
			if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") || entry.Flags&1 != 0 {
				return "", fmt.Errorf("unsafe OFD entry")
			}
			expanded += entry.UncompressedSize64
			if expanded > 100<<20 {
				return "", fmt.Errorf("OFD expansion exceeds limit")
			}
			if strings.EqualFold(cleaned, "OFD.xml") {
				foundManifest = true
			}
		}
		if !foundManifest {
			return "", fmt.Errorf("OFD manifest is missing")
		}
		return "application/ofd", nil
	default:
		return "", fmt.Errorf("unsupported invoice file type")
	}
}
