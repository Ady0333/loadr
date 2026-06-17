# Loadr

A concurrent HTTP load testing tool with live metrics and AI-powered analysis.

## Groq API key

The AI analysis uses Groq. Get a free key at https://console.groq.com.

## Run the backend

```bash
cd backend
cp .env.example .env   # then fill in your GROQ_API_KEY
go run .
```

The backend listens on `PORT` (default `8080`).

## Run the frontend

```bash
cd frontend
npm install
npm run dev
```

## Use it

Open [https://loadr-n280.onrender.com](https://loadr-n280.onrender.com/), enter a target URL, and hit **Run Test**. Watch
latency and throughput stream live, then read the AI analysis when the run ends.
