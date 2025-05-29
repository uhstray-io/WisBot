package main

import (
	"reflect"
	"testing"
)

func TestChunkString(t *testing.T) {
	input := `Market Capitalization: Measures the total value of all outstanding shares.
Trading Volume: Indicates market activity and liquidity.
Earnings Per Share (EPS): A measure of a company's profitability.
Dividend Yield: The ratio of annual dividend payments to the stock's current price.
Price-to-Earnings Ratio (P/E Ratio): Compares a stock's price to its earnings.`

	maxLength := 100
	expected := []string{"Market Capitalization: Measures the total value of all outstanding shares.",
		"Trading Volume: Indicates market activity and liquidity.",
		"Earnings Per Share (EPS): A measure of a company's profitability.",
		"Dividend Yield: The ratio of annual dividend payments to the stock's current price.",
		"Price-to-Earnings Ratio (P/E Ratio): Compares a stock's price to its earnings."}

	actual := splitStringByLength(input, maxLength)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestCodeBlock(t *testing.T) {
	input := "Market Capitalization: Measures the total value of all outstanding shares.\n" +
		"```go\n" +
		"func main() {\n" +
		"fmt.Println(\"Hello, World!\")\n" +
		"}\n" +
		"```\n" +
		"Trading Volume: Indicates market activity and liquidity.\n"

	expected := []string{"Market Capitalization: Measures the total value of all outstanding shares.",
		"```go\nfunc main() {\nfmt.Println(\"Hello, World!\")\n}\n```",
		"Trading Volume: Indicates market activity and liquidity.",
	}

	expectedBool := []bool{false, true, false}

	actual, actualBool := splitCodeBlocks(input)

	// if !reflect.DeepEqual(actual, expected) {
	// 	t.Errorf("Expected %v, got %v", expected, actual)
	// }

	for i := range actual {
		if actual[i] != expected[i] {
			t.Errorf("Expected \"%v\", \ngot \"%v\"", actual[i], expected[i])
		}
	}

	for i := range actualBool {
		if actualBool[i] != expectedBool[i] {
			t.Errorf("Expected %v, got %v", expectedBool[i], actualBool[i])
		}
	}
}
