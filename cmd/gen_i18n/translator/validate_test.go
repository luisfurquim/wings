package translator

import "testing"

func TestValidateCell_SimpleText(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		dst     string
		wantErr bool
	}{
		{
			name: "no placeholders — always ok",
			src:  "Bem-vindo ao sistema",
			dst:  "Welcome to the system",
		},
		{
			name: "single ref preserved",
			src:  "Você tem {{%count}} itens",
			dst:  "You have {{%count}} items",
		},
		{
			name: "multiple refs preserved",
			src:  "Olá, {{nome}}, você tem {{%count}} itens",
			dst:  "Hello, {{nome}}, you have {{%count}} items",
		},
		{
			name:    "ref dropped by translator",
			src:     "Você tem {{%count}} itens",
			dst:     "You have items",
			wantErr: true,
		},
		{
			name:    "all refs dropped",
			src:     "{{nome}} tem {{%count}} itens",
			dst:     "has items",
			wantErr: true,
		},
		{
			name: "empty source — always ok",
			src:  "",
			dst:  "anything",
		},
		{
			name: "duplicate ref in source — checked once",
			src:  "{{%n}} de {{%n}} resultados",
			dst:  "{{%n}} of {{%n}} results",
		},
		{
			name:    "duplicate ref in source — dropped",
			src:     "{{%n}} de {{%n}} resultados",
			dst:     "results",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCell(SimpleTextKey, tc.src, tc.dst)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateCell(%q, %q, %q) error = %v, wantErr %v",
					SimpleTextKey, tc.src, tc.dst, err, tc.wantErr)
			}
		})
	}
}

func TestValidateCell_FlexCell(t *testing.T) {
	cases := []struct {
		name    string
		cellKey string
		src     string
		dst     string
		wantErr bool
	}{
		{
			name:    "count placeholder preserved",
			cellKey: "m.one",
			src:     "o %n aluno está aprovado",
			dst:     "the %n student is approved",
		},
		{
			name:    "count placeholder dropped",
			cellKey: "m.other",
			src:     "os %n alunos estão aprovados",
			dst:     "the students are approved",
			wantErr: true,
		},
		{
			name:    "no placeholder — always ok",
			cellKey: "m.zero",
			src:     "nenhum aluno aprovado",
			dst:     "no students approved",
		},
		{
			name:    "multiple placeholders all present",
			cellKey: "f.other",
			src:     "as %n alunas de %curso estão aprovadas",
			dst:     "the %n students from %curso are approved",
		},
		{
			name:    "one of multiple placeholders dropped",
			cellKey: "f.other",
			src:     "as %n alunas de %curso estão aprovadas",
			dst:     "the %n students are approved",
			wantErr: true,
		},
		{
			name:    "empty source — always ok",
			cellKey: "m.few",
			src:     "",
			dst:     "anything",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCell(tc.cellKey, tc.src, tc.dst)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateCell(%q, %q, %q) error = %v, wantErr %v",
					tc.cellKey, tc.src, tc.dst, err, tc.wantErr)
			}
		})
	}
}

func TestExtractTemplateRefs(t *testing.T) {
	got := extractTemplateRefs("olá {{nome}}, você tem {{%count}} itens e {{nome}} novamente")
	want := []string{"{{nome}}", "{{%count}}"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractPctTokens(t *testing.T) {
	got := extractPctTokens("os %n alunos de %curso estão aprovados, %n de %n")
	want := []string{"%n", "%curso"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
