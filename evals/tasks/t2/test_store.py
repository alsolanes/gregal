import pytest
from store import add_note, get_median


def test_add():
    assert add_note([], "hola") == [{"text": "hola"}]


def test_median_odd():
    notes = add_note(add_note(add_note([], "a"), "abc"), "abcde")
    assert get_median(notes) == 3.0


def test_median_even():
    notes = add_note(add_note([], "ab"), "abcd")
    assert get_median(notes) == 3.0


def test_median_empty():
    assert get_median([]) == 0.0
