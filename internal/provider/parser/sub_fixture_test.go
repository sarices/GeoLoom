package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDetectInputTypeFromFixture(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("..", "..", "..", "test", "fixtures", "sub.txt")
	file, err := os.Open(fixturePath)
	if err != nil {
		t.Fatalf("打开脱敏样例失败: %v", err)
	}
	defer file.Close()

	var gotTypes []InputType
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		inputType, _, detectErr := DetectInputType(line)
		if detectErr != nil {
			t.Fatalf("识别输入类型失败，line=%q err=%v", line, detectErr)
		}
		gotTypes = append(gotTypes, inputType)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("读取脱敏样例失败: %v", err)
	}

	wantTypes := []InputType{InputTypeSource, InputTypeNode, InputTypeNode, InputTypeSource}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("输入类型识别不匹配: got=%v want=%v", gotTypes, wantTypes)
	}
}
