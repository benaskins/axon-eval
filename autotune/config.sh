# Tunable configuration. THIS IS THE FILE YOU EDIT.
# Modify these values to optimize the score metric.
# After editing, the eval harness will apply changes and measure results.

# Server parameters (llama-server via aurelia)
MODEL_PATH="/Users/benaskins/models/Qwen3.5-397B-A17B-MXFP4_MOE-00001-of-00006.gguf"
PARALLEL=4
CTX_SIZE=0          # 0 = model default
BATCH_SIZE=0        # 0 = server default
UBATCH_SIZE=0       # 0 = server default
THREADS=0           # 0 = server default (auto)
CACHE_TYPE_K=""     # "", "q8_0", "q4_0", "f16"
CACHE_TYPE_V=""     # "", "q8_0", "q4_0", "f16"
EXTRA_SERVER_ARGS=""

# Eval target (which server to hit)
EVAL_URL="http://localhost:8091"
EVAL_MODEL="/Users/benaskins/models/mlx/Qwen3.5-27B-8bit"

# Eval parameters
EVAL_CATEGORY=""           # empty = all categories (stratified)
EVAL_LIMIT=3               # 3 per category = 15 total
EVAL_WORKERS=1
TARGET_THROUGHPUT=0.15     # cases/sec (15 cases in ~100s)
