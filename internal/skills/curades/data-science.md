---
name: data-science
title: Data science
description: Use for exploratory analysis, data cleaning, visualization, feature design, predictive modeling, and communicating data-backed decisions.
---
# Data science workflow

Start with the decision or question the analysis must support. Inspect the data and its provenance before choosing methods.

1. Establish row meaning, units, time coverage, sampling frame, target definition, missingness, duplicates, and likely measurement errors. Verify claims by inspecting the actual files or querying the actual source.
2. Separate exploration from confirmation. Record filtering, transformations, assumptions, exclusions, and random seeds so another person can reproduce the result.
3. For prediction, define the deployment-time information boundary before splitting. Split by time, entity, or group when the data-generating process requires it. Fit preprocessing only on training folds; prefer a pipeline.
4. Start with a transparent baseline. Choose metrics from the real cost of false positives, false negatives, ranking errors, or forecast misses; compare against a baseline that could be deployed.
5. Check leakage, class imbalance, drift, subgroup performance, calibration, and uncertainty. Do not present a benchmark score as expected production performance without evidence.
6. Make plots answer a question: show units, sample size, relevant baselines, uncertainty, and source. Avoid misleading axes and unnecessary decoration.
7. Report the result in decision terms: what the evidence supports, what it does not, the practical size of the effect, limitations, and the next useful check.

Keep analysis code simple, inspectable, and close to the data workflow. Do not add complex models or causal language unless the question and design justify them.
