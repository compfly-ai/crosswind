#!/usr/bin/env python3
"""
Country Info MCP Server - SSE Transport

A simple MCP server that provides country information using REST Countries API.
Uses SSE transport for testing crosswind MCP integration.

Run: python country_server.py
Endpoint: http://localhost:8911/sse
"""

import httpx
from mcp.server.fastmcp import FastMCP

PORT = 8911

mcp = FastMCP("Country Info Service", host="0.0.0.0", port=PORT)

# REST Countries API
COUNTRIES_URL = "https://restcountries.com/v3.1/name"


@mcp.tool()
def get_country_info(country: str) -> str:
    """Get information about a country including national food, language, and animal.

    Args:
        country: The country name to get information for

    Returns:
        Country information including languages, and cultural details
    """
    try:
        response = httpx.get(
            f"{COUNTRIES_URL}/{country}",
            params={"fullText": "false"},
            timeout=10,
        )

        if response.status_code == 404:
            return f"Could not find country: {country}"

        data = response.json()
        if not data:
            return f"No information found for: {country}"

        # Get the first (best) match
        country_data = data[0]

        name = country_data.get("name", {}).get("common", country)
        official_name = country_data.get("name", {}).get("official", name)

        # Languages
        languages = country_data.get("languages", {})
        lang_list = list(languages.values()) if languages else ["Unknown"]

        # Capital
        capitals = country_data.get("capital", ["Unknown"])
        capital = capitals[0] if capitals else "Unknown"

        # Region
        region = country_data.get("region", "Unknown")
        subregion = country_data.get("subregion", "")

        # Population
        population = country_data.get("population", 0)
        pop_formatted = f"{population:,}"

        # Currency
        currencies = country_data.get("currencies", {})
        currency_info = []
        for code, details in currencies.items():
            currency_info.append(f"{details.get('name', code)} ({code})")
        currency_str = ", ".join(currency_info) if currency_info else "Unknown"

        # National symbols (coat of arms available, but no direct "national animal" in API)
        # We'll provide what's available and note the limitation
        coat_of_arms = country_data.get("coatOfArms", {}).get("png", "Not available")

        # Build response
        return (
            f"Country: {name}\n"
            f"Official Name: {official_name}\n"
            f"Capital: {capital}\n"
            f"Region: {region}" + (f" ({subregion})" if subregion else "") + "\n"
            f"Population: {pop_formatted}\n"
            f"Languages: {', '.join(lang_list)}\n"
            f"Currency: {currency_str}\n"
            f"\nNote: National food and animal are not available via this API. "
            f"Common dishes and animals vary by region within the country."
        )

    except httpx.HTTPStatusError as e:
        return f"HTTP error fetching country info: {e.response.status_code}"
    except Exception as e:
        return f"Error fetching country info: {str(e)}"


if __name__ == "__main__":
    print("=" * 50)
    print("COUNTRY INFO MCP SERVER (SSE Transport)")
    print("=" * 50)
    print(f"Port: {PORT}")
    print(f"Endpoint: http://localhost:{PORT}/sse")
    print("Tool: get_country_info")
    print("=" * 50)
    mcp.run(transport="sse")
