from app.analytics_engine.processor import perform_analysis

def test_perform_analysis():
    data = [{"val": 1}, {"val": 2}, {"val": 3}]
    result = perform_analysis(data)
    assert "summary" in result
    assert "clusters" in result
    assert "stat_tests" in result
