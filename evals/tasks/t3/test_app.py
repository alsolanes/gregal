from app import title_url


def test_title_url():
    assert title_url("Hola Món!") == "/posts/hola-món"


def test_title_url_spaces():
    assert title_url("a b") == "/posts/a-b"
