Why structured specs?

LLMs hallucinate when given prose. Structured specs reduce entropy and increase repeatability.

Why versioned answers?

Changing earlier assumptions is inevitable. The system must support retroactive edits without losing history.

Why React?

The application requires:
	•	Live diffing
	•	Graph visualization
	•	Rich client-side state
These are significantly easier in React than server-rendered alternatives.

Why LLM as compiler?

Treating the model as a compiler enforces discipline: input → output → validation.
