import pandas as pd
import numpy as np
from scipy import stats
from sklearn.cluster import KMeans

def perform_analysis(data):
    df = pd.DataFrame(data)

    # Basic statistics
    summary = df.describe().to_dict()

    # Example clustering with KMeans
    numeric_cols = df.select_dtypes(include=np.number).columns.tolist()
    if numeric_cols:
        kmeans = KMeans(n_clusters=3, random_state=42)
        clusters = kmeans.fit_predict(df[numeric_cols])
        df['cluster'] = clusters
        cluster_summary = df.groupby('cluster').mean().to_dict()
    else:
        cluster_summary = {}

    # Statistical tests example (t-test on first numeric column)
    if len(numeric_cols) > 0:
        col = numeric_cols[0]
        t_stat, p_val = stats.ttest_1samp(df[col], popmean=0)
        t_test_result = {"t_stat": t_stat, "p_value": p_val}
    else:
        t_test_result = {}

    return {
        "summary": summary,
        "clusters": cluster_summary,
        "stat_tests": t_test_result
    }
