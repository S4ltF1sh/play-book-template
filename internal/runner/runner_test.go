package runner

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// runToExit runs files under toolchain and returns (output, exitCode).
// Skips the test when the toolchain isn't installed on this machine.
func runToExit(t *testing.T, toolchain string, files []File, args []string) (string, int) {
	t.Helper()
	if _, err := Resolve(toolchain, files); err != nil {
		t.Skipf("skipping: %v", err)
	}
	s := New()
	var mu sync.Mutex
	var out strings.Builder
	exit := make(chan int, 1)
	err := s.Run(toolchain, files, args,
		func(o string) { mu.Lock(); out.WriteString(o); mu.Unlock() },
		func(string) {},
		func(code int) { exit <- code },
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case code := <-exit:
		mu.Lock()
		defer mu.Unlock()
		return out.String(), code
	case <-time.After(90 * time.Second):
		s.Kill()
		t.Fatal("program did not exit in time")
		return "", -1
	}
}

func wantOutput(t *testing.T, toolchain string, files []File, args []string, want string) {
	t.Helper()
	out, code := runToExit(t, toolchain, files, args)
	if code != 0 {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("output missing %q:\n%s", want, out)
	}
}

// C: a header must be staged for #include but never passed to the compiler.
func TestCWithHeader(t *testing.T) {
	wantOutput(t, "c", []File{
		{Name: "main.c", Content: "#include <stdio.h>\n#include \"greet.h\"\nint main(void){printf(\"%s\\n\", GREETING);return 0;}\n"},
		{Name: "greet.h", Content: "#define GREETING \"hi from c\"\n"},
	}, nil, "hi from c")
}

func TestCpp(t *testing.T) {
	wantOutput(t, "cpp", []File{
		{Name: "main.cpp", Content: "#include <iostream>\nint main(){std::cout << \"hi from c++\\n\";}\n"},
	}, nil, "hi from c++")
}

func TestPythonArgs(t *testing.T) {
	wantOutput(t, "python", []File{
		{Name: "main.py", Content: "import sys\nprint('hi from', sys.argv[1])\n"},
	}, []string{"python"}, "hi from python")
}

func TestNode(t *testing.T) {
	wantOutput(t, "node", []File{
		{Name: "main.js", Content: "console.log('hi from node');\n"},
	}, nil, "hi from node")
}

func TestJava(t *testing.T) {
	wantOutput(t, "java", []File{
		{Name: "Main.java", Content: "public class Main{public static void main(String[] a){System.out.println(\"hi from java\");}}\n"},
	}, nil, "hi from java")
}

func TestKotlin(t *testing.T) {
	wantOutput(t, "kotlin", []File{
		{Name: "main.kt", Content: "fun main(){println(\"hi from kotlin\")}\n"},
	}, nil, "hi from kotlin")
}

// Toolchain inference from the entry file's extension.
func TestResolveInference(t *testing.T) {
	tc, err := Resolve("", []File{{Name: "x.cpp"}})
	if err != nil {
		t.Skipf("c++ unavailable: %v", err)
	}
	if tc.Name != "cpp" {
		t.Fatalf("inferred %q, want cpp", tc.Name)
	}
}
