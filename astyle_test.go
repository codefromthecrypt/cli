package cli_test

import (
	"testing"

	"github.com/apexlang/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAstyle(t *testing.T) {
	expected := "#include <cstdio>\nint main() {\n    int 🦄, a, *b = a, c = 🦄 * 2, *d = nullptr;\n    return -1;\n}"
	code := "#include <cstdio>\nint main(){int 🦄,a,*b=a,c=🦄*2,*d=nullptr;return -1;}"
	formatted, err := cli.Astyle(code, "pad-oper style=google")
	require.NoError(t, err)
	assert.Equal(t, expected, formatted)
}
