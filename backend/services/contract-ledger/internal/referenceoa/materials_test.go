package referenceoa

import (
	"strings"
	"testing"
)

func TestMaterialReferenceStrictIdentityAndMetadata(t *testing.T) {
	tenant := "22222222-2222-4222-8222-222222222222"
	id := "33333333-3333-4333-8333-333333333333"
	valid := FileReference{"1", "evidence-service", "evidence-file", id, id, tenant, 2, 2, "available", "合同.pdf", 5, "application/pdf", strings.Repeat("a", 64)}
	if e := validateFile(valid, tenant, id, 2); e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*FileReference){"tenant": func(v *FileReference) { v.TenantID = id }, "version": func(v *FileReference) { v.Version = 1 }, "revision": func(v *FileReference) { v.Revision = 1 }, "source": func(v *FileReference) { v.Source = "contract-ledger" }, "pending": func(v *FileReference) { v.Status = "pending" }, "digest": func(v *FileReference) { v.SHA256 = "no" }, "size": func(v *FileReference) { v.Size = -1 }, "empty_name": func(v *FileReference) { v.Name = " " }} {
		t.Run(name, func(t *testing.T) {
			v := valid
			change(&v)
			if validateFile(v, tenant, id, 2) == nil {
				t.Fatal("invalid reference accepted")
			}
		})
	}
}
