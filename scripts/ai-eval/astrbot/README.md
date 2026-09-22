# AstrBot component evaluation

This harness imports the unchanged upstream `ToolLoopAgentRunner`, `ProviderOpenAIOfficial` and `FunctionToolExecutor`. The only replacements are a loopback HTTP model fixture and fictional application tools. It is **not** a deployed AstrBot OpenAPI-to-QQ test and does not use real model credentials.

Source revision: `95e98b8aed75d56713666eff39e31bafffd95426` (AstrBot 4.28.1). Tested interpreter: CPython 3.12.9 on macOS arm64.

Set `ASTRBOT_SOURCE` to a checkout at that revision. `ASTRBOT_EVAL_OUTPUT` selects an isolated output and runtime-data directory. Both paths default to the temporary directories used for this evaluation. Run `probe.py` with an isolated Python environment containing upstream dependencies. Source files are not patched.

The JSON output records positive and negative observations. A zero exit status means the harness executed its assertions; it does **not** mean every candidate behavior passed the application's acceptance criteria. In particular, inspect `model_repeats_identical_tool.deduplicated`, `permanent_503_after_tool.exposes_provider_detail`, and `literal_tool_markup.suppressed`.

The fake model returns scripted responses and does not measure summary quality. A caller supplying isolated conversation histories is not evidence of a persistent session store's access control.
