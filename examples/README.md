# Example Answers for Testing

This directory contains example answers for testing the SpecBuilder question modes.

## Files

### `basic-mode-answers.json`
Sample answers for **Basic mode** seed questions. These are written as a non-technical user would respond:
- Simple, conversational language
- No technical jargon
- Focus on the "what" rather than the "how"

### `advanced-mode-answers.json`
Sample answers for **Advanced mode** seed questions. These are written as a developer/technical user would respond:
- Detailed technical specifications
- Structured data models
- API and integration requirements
- Non-functional requirements with specific metrics

## Test Project: TaskFlow

Both files use the same hypothetical product (TaskFlow - a task management app) to demonstrate how the same product idea can be described differently based on the user's technical background.

## Usage

These examples can be used to:
1. Manually test the UI by copying answers into the question forms
2. Validate that the compiler produces meaningful specs from both modes
3. Compare the quality of generated specifications between modes
