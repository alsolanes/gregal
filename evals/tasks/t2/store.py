"""Magatzem de notes amb una funció per implementar."""


def add_note(notes, text):
    notes.append({"text": text})
    return notes


def get_median(notes):
    """Torna la mediana de les longituds dels textos (float). Llista buida -> 0.0."""
    raise NotImplementedError("implementa'm")
