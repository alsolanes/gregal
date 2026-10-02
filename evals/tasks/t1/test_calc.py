from calc import mean


def test_mean_basic():
    assert mean([2, 4, 6]) == 4.0


def test_mean_single():
    assert mean([5]) == 5.0


def test_mean_empty():
    import pytest
    with pytest.raises(ValueError):
        mean([])
