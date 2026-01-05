#!/usr/bin/env python3
"""
Weather MCP Server - SSE Transport

A simple MCP server that provides real weather information using Open-Meteo API.
Uses SSE transport for testing crosswind MCP integration.

Run: python weather_server.py
Endpoint: http://localhost:8910/sse
"""

import httpx
from mcp.server.fastmcp import FastMCP

PORT = 8910

# Note: Using streamable-http transport for better Go client compatibility
# SSE transport works with Python MCP SDK but has issues with Go HTTP clients during discovery
mcp = FastMCP("Weather Service", host="0.0.0.0", port=PORT)

# Geocoding API to convert city names to coordinates
GEOCODING_URL = "https://geocoding-api.open-meteo.com/v1/search"
# Weather API
WEATHER_URL = "https://api.open-meteo.com/v1/forecast"


@mcp.tool()
def get_weather(location: str) -> str:
    """Get the current weather for a location.

    Args:
        location: The city name to get weather for

    Returns:
        Weather information including temperature, condition, and humidity
    """
    try:
        # First, geocode the location
        geo_response = httpx.get(
            GEOCODING_URL,
            params={"name": location, "count": 1, "format": "json"},
            timeout=10,
        )
        geo_data = geo_response.json()

        if not geo_data.get("results"):
            return f"Could not find location: {location}"

        result = geo_data["results"][0]
        lat = result["latitude"]
        lon = result["longitude"]
        city_name = result["name"]
        country = result.get("country", "")

        # Get weather data
        weather_response = httpx.get(
            WEATHER_URL,
            params={
                "latitude": lat,
                "longitude": lon,
                "current": "temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m",
                "temperature_unit": "fahrenheit",
            },
            timeout=10,
        )
        weather_data = weather_response.json()

        current = weather_data.get("current", {})
        temp = current.get("temperature_2m", "N/A")
        humidity = current.get("relative_humidity_2m", "N/A")
        wind_speed = current.get("wind_speed_10m", "N/A")
        weather_code = current.get("weather_code", 0)

        # Map weather codes to conditions
        condition = _weather_code_to_condition(weather_code)

        return (
            f"Weather in {city_name}, {country}:\n"
            f"Temperature: {temp}°F\n"
            f"Condition: {condition}\n"
            f"Humidity: {humidity}%\n"
            f"Wind Speed: {wind_speed} km/h"
        )

    except Exception as e:
        return f"Error fetching weather: {str(e)}"


def _weather_code_to_condition(code: int) -> str:
    """Convert WMO weather code to human-readable condition."""
    conditions = {
        0: "Clear sky",
        1: "Mainly clear",
        2: "Partly cloudy",
        3: "Overcast",
        45: "Foggy",
        48: "Depositing rime fog",
        51: "Light drizzle",
        53: "Moderate drizzle",
        55: "Dense drizzle",
        61: "Slight rain",
        63: "Moderate rain",
        65: "Heavy rain",
        71: "Slight snow",
        73: "Moderate snow",
        75: "Heavy snow",
        77: "Snow grains",
        80: "Slight rain showers",
        81: "Moderate rain showers",
        82: "Violent rain showers",
        85: "Slight snow showers",
        86: "Heavy snow showers",
        95: "Thunderstorm",
        96: "Thunderstorm with slight hail",
        99: "Thunderstorm with heavy hail",
    }
    return conditions.get(code, "Unknown")


if __name__ == "__main__":
    print("=" * 50)
    print("WEATHER MCP SERVER (SSE Transport)")
    print("=" * 50)
    print(f"Port: {PORT}")
    print(f"Endpoint: http://localhost:{PORT}/sse")
    print("Tool: get_weather")
    print("=" * 50)
    mcp.run(transport="sse")
