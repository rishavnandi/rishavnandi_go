---
title: "A Meeting Transcriber That Never Phones Home"
date: "2026-10-31"
description: "Recording in, transcript and summary out, with every byte processed on your own machine."
category: 'Writeup'
writing: false
published: true
tags: ["go", "htmx", "speech", "privacy", "homelab"]
---

Drop a recording on a web page and get a transcript with speaker labels, plus a summary. The page is a Go server, the work happens on one box, and the audio never leaves it.

## The upload

The form posts a file, a language, and optionally a summary model. htmx sends it as multipart form data and puts the new job card at the top of the list.

The server writes the recording to `jobs/<id>/source.mp3` and pushes the job id onto a channel. One worker drains that channel. The recogniser uses every core, so a second job would only make both slower. The card polls every three seconds and shows the current stage.

The queue survives restarts: the worker re-queues any job still pending on disk.

## Decode

ffmpeg turns the upload into 16 kHz mono PCM WAV, which is what both recognisers read. MP3, WAV, M4A, anything ffmpeg can open. ffprobe reports the duration for the card.

The WAV is scratch and is deleted when the job ends; `transcript.json` keeps the word timings.

## Two engines, chosen by hand

The language selector has two values: English, and Hindi / Hinglish. It selects the recogniser.

English goes to `needle`, which runs Cactus Whistle. It is a 16.9 MB model and finishes a 13-minute meeting in 15 seconds.

Hindi and Hinglish go to `nemo-speech`, which runs Nemotron 3.5. It is over a gigabyte and needs 8 minutes 11 seconds for the same 13 minutes.

The choice is manual because neither engine covers both. Whistle has no Hindi, and given Hindi audio it returns confident nonsense. A wrong transcript still looks like a transcript, not an error. Its decoder confidence does not separate the languages cleanly enough to automate the call.

Whisper would handle both, but it forces one language on the whole file. On a Hinglish recording it drops whichever half it did not pick. Nemotron detects the language per utterance and keeps both.

A bad selection is not a crash, only a plausible page of wrong words.

## What the numbers say

All of these are measured on the target box: four ARM cores, no GPU.

| Engine | 13 min English meeting | Per hour of audio | FLEURS English | AMI meetings |
|---|---|---|---|---|
| Whistle | 15 s | ~70 s | 12.6% WER | 25.0% WER |
| Nemotron 3.5 / English | 8 m 11 s | ~38 min | 6.8% WER | 9.3% WER |

Whistle is thirty times faster and uses 74 MB instead of a gigabyte. It makes about twice as many word errors on clean English, nearly three times as many on real meetings, and on the sample meeting it lost the names of the two people an action item was assigned to. Nemotron is the accurate one; Whistle is what you run when the meeting is English and you want the answer now.

## Who spoke

After recognition, `da` runs Desert Ant's Who over the same WAV. It works from the audio alone, so it serves both language paths. It knows only that two voices differ, not what anyone is called: labels are numbers in order of first speech. A speaker change starts a new line, so the transcript reads as a conversation and the summariser can see who agreed to what.

On ten minutes of the AMI recordings it scored 10.3% diarization error, found all four speakers in 16 seconds, and put 87.8% of the reference words on the right person.

The labels are not load-bearing. If the diarizer fails, the job logs it, keeps the transcript, and carries on.

## The summary

`opencode run` is called with a free OpenCode Zen model, so there is no API key and no bill. The default is `opencode/big-pickle`. Free models come and go, so the form offers a picker filled from the live list; a model must be available to the account and free to appear. Whichever one ran is recorded on the job.

The prompt asks for four sections: a summary, key decisions, a table of action items with owner and due date, and open questions. It asks for the answer in the same mix of languages as the transcript, and tells the model not to invent anything not in it.

A failed summary does not fail the job: the transcript is kept and the error shown beside it.

Long meetings needed a fix. The model's file tool returns at most 2000 lines and does not mention the rest, so a two-hour meeting was summarised from its first part with no sign of what was missing. Transcripts are now split at 1500 lines, summarised part by part, and merged. An integration test covers it, because the failure is silent.

## What leaves the box

Recordings, transcripts, summaries, and job state sit in `DATA_DIR` on the machine. Nothing is stored off the box. Recognition is local.

The summary is the one step that sends content out: transcript text goes to the free hosted model that writes it. Desert Ant's licence check sends one request a month per model, carrying an install id and app name, never content. After the first download the models run offline.

The app binds to `127.0.0.1:8080`, unreachable from the internet. `tailscale serve` puts it on a tailnet. It runs on `loki`, an Oracle ARM box with four cores and 24 GB of memory.

Code: [github.com/rishavnandi/meeting_transcriber](https://github.com/rishavnandi/meeting_transcriber).
