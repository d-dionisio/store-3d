package editorial

import "testing"

func TestInputValidation(t *testing.T) {
	valid := Input{Slug: "casa-contemporanea", Title: "Casa contemporânea", Description: "Uma edição.", Eyebrow: "Edição digital"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid input: %v", err)
	}
	for _, slug := range []string{"", "Com Espaço", "duplicado_", "-inicio"} {
		value := valid
		value.Slug = slug
		if value.Validate() == nil {
			t.Errorf("accepted invalid slug %q", slug)
		}
	}
}
