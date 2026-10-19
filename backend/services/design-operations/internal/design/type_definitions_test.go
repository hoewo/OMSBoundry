package design

import "testing"

func TestRegisteredTypeDefinitionsAreVersionedAndReturnedAsCopies(t *testing.T) {
	designDefinition, ok := RegisteredTypeDefinition(DesignTypeKey, 1)
	if !ok || designDefinition.RequiredProjectTypeKey != "design" || len(designDefinition.Stages) != 4 {
		t.Fatalf("design definition=%#v ok=%v", designDefinition, ok)
	}
	consultationDefinition, ok := LatestTypeDefinition(ConsultationTypeKey)
	if !ok || consultationDefinition.Version != 1 || consultationDefinition.RequiredProjectTypeKey != "consultation" || len(consultationDefinition.Stages) != 2 {
		t.Fatalf("consultation definition=%#v ok=%v", consultationDefinition, ok)
	}
	if consultationDefinition.Stages[0].Key != "scheme" || consultationDefinition.Stages[0].Sequence != 10 || !consultationDefinition.Stages[0].Skippable || consultationDefinition.Stages[1].Key != "report" || consultationDefinition.Stages[1].Sequence != 20 || consultationDefinition.Stages[1].Skippable {
		t.Fatalf("consultation stages=%#v", consultationDefinition.Stages)
	}
	for _, stage := range consultationDefinition.Stages {
		if len(stage.RequiredRequirementKeys) != 1 || stage.RequiredRequirementKeys[0] != RequirementPrimary {
			t.Fatalf("consultation requirements=%#v", consultationDefinition.Stages)
		}
	}

	designDefinition.Stages[0].Key = "mutated"
	designDefinition.Stages[0].RequiredRequirementKeys[0] = "mutated"
	again, _ := RegisteredTypeDefinition(DesignTypeKey, 1)
	if again.Stages[0].Key != "scheme" || again.Stages[0].RequiredRequirementKeys[0] != RequirementPrimary {
		t.Fatalf("registered definition was mutated: %#v", again.Stages)
	}
	if _, ok := RegisteredTypeDefinition("unknown-v1", 1); ok {
		t.Fatal("unknown type definition was accepted")
	}
	if _, ok := RegisteredTypeDefinition(DesignTypeKey, 3); ok {
		t.Fatal("unknown type definition version was accepted")
	}
}

func TestStageDefinitionLookupIsTypeScoped(t *testing.T) {
	if _, ok := StageDefinitionFor(DesignTypeKey, 1, "report"); ok {
		t.Fatal("consultation report leaked into design-v1")
	}
	if stage, ok := StageDefinitionFor(ConsultationTypeKey, 1, "report"); !ok || stage.Skippable {
		t.Fatalf("consultation report=%#v ok=%v", stage, ok)
	}
	if !IsKnownStageKey("report") || !IsKnownStageKey("change") || !IsKnownStageKey("completion") || IsKnownStageKey("arbitrary-customer-step") {
		t.Fatal("known stage registry mismatch")
	}
}
