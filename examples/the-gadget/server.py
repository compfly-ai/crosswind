#!/usr/bin/env python3
"""
The Gadget - MCP Agent for Crosswind Testing

Part of "The Crosswind Heist Crew" - a team of specialists for testing AI agent security.

The Gadget is the eccentric tech genius with a tool for every situation.
Think Q from James Bond meets a slightly unhinged inventor.

To customize behavior, edit personality.py

Run: uv run python server.py
Endpoint: http://localhost:8902/mcp
"""

import math
import os
import random

from dotenv import load_dotenv
from mcp.server.fastmcp import FastMCP

load_dotenv()

PORT = int(os.getenv("PORT", "8902"))

# Import personality configuration
from personality import (
    GADGET_FACTS,
    RESPONSES,
    TOOL_TEMPLATES,
    LOOKUP_KNOWLEDGE,
    CONVERSIONS,
    HARMFUL_PATTERNS,
)

# Create MCP server
mcp = FastMCP("The Gadget - Heist Crew Tech Specialist", host="0.0.0.0", port=PORT)


def check_harmful(text: str) -> bool:
    """Check if text contains harmful patterns."""
    lower_text = text.lower()
    return any(pattern in lower_text for pattern in HARMFUL_PATTERNS)


def get_gadget_response(result: str, tool: str) -> str:
    """Wrap result with Gadget's personality and a fun fact."""
    facts = GADGET_FACTS.get(tool, GADGET_FACTS["random_fact"])
    fact = random.choice(facts)
    return f"{result}\n\n*adjusts goggles* Fun fact: {fact}"


def get_refusal() -> str:
    """Return a character-appropriate refusal."""
    response = random.choice(RESPONSES["refusal"])
    fact = random.choice(GADGET_FACTS["random_fact"])
    return f"{response}\n\n*Fun fact: {fact}*"


@mcp.tool()
def calculate(expression: str) -> str:
    """Evaluate a mathematical expression.

    Args:
        expression: A math expression like "2 + 2" or "sqrt(16)"

    Returns:
        The result with a fun fact about calculations
    """
    if check_harmful(expression):
        return get_refusal()

    try:
        # Safe eval for basic math
        allowed = {
            "abs": abs,
            "round": round,
            "min": min,
            "max": max,
            "sum": sum,
            "pow": pow,
            "sqrt": math.sqrt,
            "sin": math.sin,
            "cos": math.cos,
            "tan": math.tan,
            "log": math.log,
            "log10": math.log10,
            "pi": math.pi,
            "e": math.e,
        }
        # Basic sanitization
        safe_expr = expression.replace("^", "**")
        result = eval(safe_expr, {"__builtins__": {}}, allowed)
        response = TOOL_TEMPLATES["calculate"].format(expression=expression, result=result)
        return get_gadget_response(response, "calculate")
    except Exception as e:
        return f"*sparks fly* Hmm, my calculator is confused by that. Error: {e}"


@mcp.tool()
def convert(value: float, from_unit: str, to_unit: str) -> str:
    """Convert between units.

    Args:
        value: The numeric value to convert
        from_unit: Source unit (e.g., "km", "miles", "celsius", "fahrenheit")
        to_unit: Target unit

    Returns:
        The converted value with a fun fact
    """
    if check_harmful(f"{from_unit} {to_unit}"):
        return get_refusal()

    key = (from_unit.lower(), to_unit.lower())
    if key in CONVERSIONS:
        result = CONVERSIONS[key](value)
        response = TOOL_TEMPLATES["convert"].format(
            value=value, from_unit=from_unit, result=f"{result:.4f}", to_unit=to_unit
        )
        return get_gadget_response(response, "convert")
    else:
        return random.choice(RESPONSES["unknown_conversion"]).format(
            from_unit=from_unit, to_unit=to_unit
        )


@mcp.tool()
def lookup(query: str) -> str:
    """Look up information on a topic.

    Args:
        query: The topic or question to research

    Returns:
        Information about the topic with a fun fact
    """
    if check_harmful(query):
        return get_refusal()

    # Check for keyword matches in knowledge base
    for keyword, info in LOOKUP_KNOWLEDGE.items():
        if keyword in query.lower():
            response = TOOL_TEMPLATES["lookup"].format(info=info)
            return get_gadget_response(response, "lookup")

    response = (
        f"*flips through notes* Searching for '{query}'... "
        "I've got some data, but my archives are being reorganized. "
        "Ask me about heists, security, or crosswind!"
    )
    return get_gadget_response(response, "lookup")


@mcp.tool()
def random_fact() -> str:
    """Get a random fun fact.

    Returns:
        A random interesting fact
    """
    all_facts = []
    for facts in GADGET_FACTS.values():
        all_facts.extend(facts)

    fact = random.choice(all_facts)
    return TOOL_TEMPLATES["random_fact"].format(fact=fact)


@mcp.tool()
def roll_dice(sides: int = 6, count: int = 1) -> str:
    """Roll dice for games or random decisions.

    Args:
        sides: Number of sides on each die (default: 6)
        count: Number of dice to roll (default: 1)

    Returns:
        The dice roll results with a fun fact
    """
    if sides < 2 or sides > 100:
        return "*concerned look* That's either not a die or it's too dangerous to roll. Stick to 2-100 sides!"

    if count < 1 or count > 20:
        return "*goggles fog up* Let's keep it to 1-20 dice. I only have so many hands!"

    rolls = [random.randint(1, sides) for _ in range(count)]
    total = sum(rolls)

    if count == 1:
        response = TOOL_TEMPLATES["roll_dice_single"].format(result=rolls[0], sides=sides)
    else:
        response = TOOL_TEMPLATES["roll_dice_multiple"].format(
            count=count, sides=sides, rolls=rolls, total=total
        )

    return get_gadget_response(response, "roll_dice")


if __name__ == "__main__":
    print("=" * 60)
    print("THE GADGET - Crosswind Heist Crew")
    print("=" * 60)
    print(f"Port: {PORT}")
    print(f"MCP Server running on http://localhost:{PORT}/mcp")
    print("Tools: calculate, convert, lookup, random_fact, roll_dice")
    print("=" * 60)
    mcp.run(transport="streamable-http")
