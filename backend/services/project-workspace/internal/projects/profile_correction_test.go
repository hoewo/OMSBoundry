package projects

import (
	"context"
	"strings"
	"testing"
)

func TestProfileCorrectionValidatesOwnedFieldsWithoutExternalCalls(t *testing.T) {
	for _, node := range []string{NodeTypeProject, NodeTypePackage} {
		t.Run(node, func(t *testing.T) {
			store := &storeStub{details: Details{Project: Project{ID: testProject, NodeType: node, Status: StatusActive}}}
			service, _ := NewService(store, &referencesStub{})
			number := "OA-EDIT-01"
			if node == NodeTypePackage {
				number = ""
			}
			input := ProfileCorrectionInput{UpdateInput: UpdateInput{CreateInput: CreateInput{CommandID: testCommand, ProjectNumber: number, Name: " 名称更正 ", ProjectTypeKeys: []string{" DESIGN "}}, ExpectedRevision: 3}, CorrectionReason: " 修改项目基本资料 "}
			if _, err := service.CorrectProfile(context.Background(), Actor{TenantID: testTenant}, testProject, input); err != nil {
				t.Fatal(err)
			}
			if store.correctionCalls != 1 || store.updateCalls != 0 || store.correctionInput.Name != "名称更正" || store.correctionInput.CorrectionReason != "修改项目基本资料" || store.correctionInput.ProjectTypeKeys[0] != "design" {
				t.Fatalf("unexpected normalized command: %#v", store.correctionInput)
			}
			for _, invalid := range []string{"reason", "long reason", "types", "revision", "client party"} {
				bad := input
				switch invalid {
				case "reason":
					bad.CorrectionReason = " "
				case "long reason":
					bad.CorrectionReason = strings.Repeat("更", 1001)
				case "types":
					bad.ProjectTypeKeys = nil
				case "revision":
					bad.ExpectedRevision = 0
				case "client party":
					bad.ClientParty = &PartyReferenceInput{PartyID: testSubject, SourceRevision: 1}
				}
				if _, err := service.CorrectProfile(context.Background(), Actor{TenantID: testTenant}, testProject, bad); err == nil {
					t.Fatalf("accepted %s", invalid)
				}
			}
			if store.correctionCalls != 1 {
				t.Fatal("invalid command reached storage")
			}
		})
	}
}
