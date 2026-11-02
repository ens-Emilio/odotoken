package guard

import (
	"testing"
)

func TestExtractJudgeScore_JSON(t *testing.T) {
	resp := []byte(`{"score": 0.85, "reason": "good"}`)
	r := ExtractJudgeScore(resp)
	if !r.Found || r.Score != 0.85 {
		t.Errorf("got %+v, want score=0.85 found=true", r)
	}
}

func TestExtractJudgeScore_JSONString(t *testing.T) {
	resp := []byte(`{"score": "4.2"}`)
	r := ExtractJudgeScore(resp)
	if !r.Found || r.Score != 4.2 {
		t.Errorf("got %+v, want score=4.2 found=true", r)
	}
}

func TestExtractJudgeScore_Markdown(t *testing.T) {
	resp := []byte("## Evaluation\nThe response is good.\n\"score\": 3.7\n")
	r := ExtractJudgeScore(resp)
	if !r.Found || r.Score != 3.7 {
		t.Errorf("got %+v, want score=3.7 found=true", r)
	}
}

func TestExtractJudgeScore_NoScore(t *testing.T) {
	resp := []byte(`{"result": "ok"}`)
	r := ExtractJudgeScore(resp)
	if r.Found {
		t.Errorf("expected no score, got %+v", r)
	}
}

func TestExtractJudgeScore_InvalidJSON(t *testing.T) {
	resp := []byte(`not json at all`)
	r := ExtractJudgeScore(resp)
	if r.Found {
		t.Errorf("expected no score for invalid input, got %+v", r)
	}
}