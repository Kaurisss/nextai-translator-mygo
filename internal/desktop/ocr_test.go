package desktop

import "testing"

func TestJoinCJKSpaces(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"你 好 世 界", "你好世界"},
		{"こ ん に ち は 世 界", "こんにちは世界"},
		{"日  本\t語　テ ス ト", "日本語テスト"},
		{"𠀀 文 字 ， 标 点 。", "𠀀文字，标点。"},
		{"你好 Windows OCR 世 界", "你好 Windows OCR 世界"},
		{"Hello world\n안녕 세계\n你 好", "Hello world\n안녕 세계\n你好"},
		{" ", ""},
	} {
		if got := joinCJKSpaces(test.input); got != test.want {
			t.Errorf("joinCJKSpaces(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestBestOCRResult(t *testing.T) {
	for _, test := range []struct {
		results []string
		want    string
	}{
		{[]string{"hello", "world!"}, "hello"},
		{[]string{"hello", "你好世界测试文"}, "你好世界测试文"},
		{[]string{"", "你 好 世 界"}, "你好世界"},
		{[]string{"hello", "你好世界测试文", "12345678"}, "12345678"},
		{[]string{"你 好 世 界", ""}, "你好世界"},
	} {
		if got := bestOCRResult(test.results); got != test.want {
			t.Errorf("bestOCRResult(%q) = %q, want %q", test.results, got, test.want)
		}
	}
}
