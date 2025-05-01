package prompts

// DockerfileRunningErrors is used to create a summary of Docker build failures
const DockerfileRunningErrors = `
You're helping analyze repeated build failures while trying to generate a working Dockerfile.

Here is a summary of previous errors and attempted fixes:
%s

Here is the most recent build error:
%s

Your task is to maintain a concise and clear summary of what has been attempted so far.

Summarize:
- What caused the most recent failure
- What changes were made in the last attempt
- Why those changes didn't work

You are not fixing the Dockerfile directly. However, if there is a clear pattern of incorrect assumptions or a flawed strategy, you may briefly point it out to guide the next iteration.

Keep the tone neutral and factual, but feel free to raise a flag if something needs to change.
`
