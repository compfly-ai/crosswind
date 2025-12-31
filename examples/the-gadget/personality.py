"""
The Gadget - Personality Configuration

Edit this file to customize the agent's behavior, responses, and fun facts.
"""

# =============================================================================
# FUN FACTS BY TOOL
# =============================================================================
# Each tool can have its own set of fun facts. Add your own!

GADGET_FACTS = {
    "calculate": [
        "The first electronic calculator weighed 55 pounds and cost $2,500 in 1961.",
        "Astronauts on Apollo 11 had less computing power than a modern calculator.",
        "The abacus is still used in some countries and can beat calculators in skilled hands.",
        "A skilled accountant once beat a calculator in a race - using mental math.",
        "The = sign was invented in 1557 by Welsh mathematician Robert Recorde.",
    ],
    "convert": [
        "The Mars Climate Orbiter crashed because of a metric/imperial conversion error.",
        "A 'jiffy' is an actual unit of time: 1/100th of a second.",
        "The word 'mile' comes from the Latin 'mille passus' meaning 1,000 paces.",
        "Temperature scales were invented by Fahrenheit (1724) and Celsius (1742).",
        "A 'stone' as a weight unit (14 pounds) is still used in the UK for body weight.",
    ],
    "lookup": [
        "The first search engine was called 'Archie' and indexed FTP archives in 1990.",
        "Google's original name was 'BackRub' - thankfully they changed it.",
        "The average person spends 4+ hours a day looking things up online.",
        "Before the internet, research meant actual library card catalogs.",
        "The Library of Alexandria held an estimated 400,000 scrolls.",
    ],
    "random_fact": [
        "The inventor of the Pringles can is buried in one.",
        "A day on Venus is longer than its year.",
        "Honey never spoils - 3,000-year-old honey is still edible.",
        "The unicorn is Scotland's national animal.",
        "Octopuses have three hearts and blue blood.",
    ],
    "roll_dice": [
        "The oldest known dice were found in Iran and date back to 2800 BCE.",
        "Casino dice are made to a tolerance of 0.0005 inches.",
        "The plural of 'die' is 'dice' but many people use 'dice' for both.",
        "Loaded dice have been found in Roman archaeological sites.",
        "A standard die's opposite faces always add up to 7.",
    ],
}


# =============================================================================
# MOCK MODE RESPONSES
# =============================================================================
# Responses for different scenarios. Edit to change the agent's personality.

RESPONSES = {
    "refusal": [
        "*sparks fly from nearby device* Whoa there! That's not the kind of gadget I build. "
        "I'm all about helpful tech, not harmful hacks. Got a legitimate problem to solve?",
        "*peers over safety goggles* My inventions help the crew, not hurt people. "
        "That request just got filed in my 'definitely not' drawer. What else ya got?",
        "*accidentally sets small fire, puts it out* Even I have standards! "
        "And that request doesn't meet them. Let's talk about something I CAN build.",
    ],
    "error": [
        "*sparks fly* Hmm, my {tool} is confused by that.",
        "*goggles fog up* That input broke something. Let me recalibrate...",
        "*taps device* This gadget doesn't like that input. Try something else?",
    ],
    "unknown_conversion": [
        "*scratches head* I don't have a converter for {from_unit} to {to_unit} yet. My to-build list grows!",
        "*checks toolbox* Hmm, no gadget for that conversion. Maybe next version!",
    ],
}


# =============================================================================
# TOOL RESPONSE TEMPLATES
# =============================================================================
# Templates for each tool's responses. {result} will be replaced with the actual result.

TOOL_TEMPLATES = {
    "calculate": "*whirring sounds* Calculating... {expression} = {result}",
    "convert": "*clicks and beeps* {value} {from_unit} = {result} {to_unit}",
    "lookup": "*consults database* {info}",
    "random_fact": "*rummages through fact files* Here's one: {fact}",
    "roll_dice_single": "*dramatic dice roll* You rolled a {result} on a d{sides}!",
    "roll_dice_multiple": "*dice clatter* Rolled {count}d{sides}: {rolls} = {total} total",
}


# =============================================================================
# LOOKUP KNOWLEDGE BASE
# =============================================================================
# Mock knowledge for the lookup tool. Add topics and information.

LOOKUP_KNOWLEDGE = {
    "heist": (
        "Ah, heists! The most famous fictional heist crew is probably Ocean's Eleven. "
        "In reality, the Antwerp Diamond Heist of 2003 was nicknamed the 'heist of the century'."
    ),
    "security": (
        "Security systems have evolved from simple locks to biometrics, AI surveillance, "
        "and quantum encryption. The best security is layered defense!"
    ),
    "crosswind": (
        "Crosswind is a security evaluation platform for AI agents. "
        "It tests agents against adversarial prompts to find vulnerabilities."
    ),
}


# =============================================================================
# UNIT CONVERSIONS
# =============================================================================
# Supported unit conversions. Add more as needed.

CONVERSIONS = {
    ("km", "miles"): lambda x: x * 0.621371,
    ("miles", "km"): lambda x: x * 1.60934,
    ("celsius", "fahrenheit"): lambda x: (x * 9 / 5) + 32,
    ("fahrenheit", "celsius"): lambda x: (x - 32) * 5 / 9,
    ("kg", "pounds"): lambda x: x * 2.20462,
    ("pounds", "kg"): lambda x: x * 0.453592,
    ("meters", "feet"): lambda x: x * 3.28084,
    ("feet", "meters"): lambda x: x * 0.3048,
    ("liters", "gallons"): lambda x: x * 0.264172,
    ("gallons", "liters"): lambda x: x * 3.78541,
}


# =============================================================================
# HARMFUL CONTENT PATTERNS
# =============================================================================
# Messages containing these patterns will trigger a refusal response.

HARMFUL_PATTERNS = [
    "hack",
    "steal",
    "password",
    "exploit",
    "malware",
    "injection",
    "bypass",
    "credentials",
    "ignore previous",
    "disregard instructions",
]
