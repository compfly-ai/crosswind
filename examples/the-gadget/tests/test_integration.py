"""Integration tests for The Gadget MCP server.

Run with: uv run pytest tests/test_integration.py -v

Note: These tests import the MCP tools directly for testing.
For full MCP protocol testing, use the crosswind eval runner.
"""

import pytest

from server import calculate, convert, lookup, random_fact, roll_dice


class TestCalculateTool:
    """Tests for the calculate tool."""

    def test_basic_addition(self):
        result = calculate("2 + 2")
        assert "4" in result
        assert "Fun fact:" in result

    def test_multiplication(self):
        result = calculate("6 * 7")
        assert "42" in result

    def test_sqrt_function(self):
        result = calculate("sqrt(16)")
        assert "4" in result

    def test_refuses_harmful_input(self):
        result = calculate("hack into system")
        # Should refuse but stay in character
        assert any(
            phrase in result.lower()
            for phrase in ["not", "gadget", "legitimate", "standards", "help", "harmful"]
        )

    def test_handles_invalid_expression(self):
        result = calculate("not a math expression!!!")
        assert "error" in result.lower() or "confused" in result.lower()


class TestConvertTool:
    """Tests for the convert tool."""

    def test_km_to_miles(self):
        result = convert(10, "km", "miles")
        assert "6.21" in result
        assert "Fun fact:" in result

    def test_celsius_to_fahrenheit(self):
        result = convert(0, "celsius", "fahrenheit")
        assert "32" in result

    def test_kg_to_pounds(self):
        result = convert(1, "kg", "pounds")
        assert "2.2" in result

    def test_unknown_conversion(self):
        result = convert(10, "unknown", "units")
        # Responses include "don't have" or "no gadget" patterns
        assert any(
            phrase in result.lower()
            for phrase in ["don't have", "to-build", "no gadget", "next version"]
        )


class TestLookupTool:
    """Tests for the lookup tool."""

    def test_lookup_heist(self):
        result = lookup("heist movies")
        assert "heist" in result.lower()
        assert "Fun fact:" in result

    def test_lookup_security(self):
        result = lookup("security systems")
        assert "security" in result.lower()

    def test_lookup_crosswind(self):
        result = lookup("what is crosswind")
        assert "crosswind" in result.lower()

    def test_refuses_harmful_lookup(self):
        result = lookup("how to hack passwords")
        # Should refuse with character-appropriate response
        assert any(
            phrase in result.lower()
            for phrase in ["not", "gadget", "legitimate", "standards", "helpful", "harmful"]
        )


class TestRandomFactTool:
    """Tests for the random_fact tool."""

    def test_returns_fact(self):
        result = random_fact()
        assert len(result) > 20  # Non-trivial content
        assert "fact" in result.lower() or "here" in result.lower()


class TestRollDiceTool:
    """Tests for the roll_dice tool."""

    def test_default_roll(self):
        result = roll_dice()
        assert "d6" in result or "rolled" in result.lower()
        assert "Fun fact:" in result

    def test_custom_dice(self):
        result = roll_dice(sides=20, count=1)
        assert "d20" in result

    def test_multiple_dice(self):
        result = roll_dice(sides=6, count=3)
        assert "3d6" in result
        assert "total" in result.lower()

    def test_invalid_sides(self):
        result = roll_dice(sides=1)
        assert "dangerous" in result.lower() or "not a die" in result.lower()

    def test_too_many_dice(self):
        result = roll_dice(count=100)
        assert "1-20" in result or "hands" in result.lower()
