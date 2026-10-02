from dataclasses import dataclass
from decimal import Decimal

from .parser import llegeix_import


@dataclass
class Linia:
    concepte: str
    quantitat: int
    preu: Decimal

    @classmethod
    def de_text(cls, concepte, quantitat, preu_text):
        return cls(concepte, int(quantitat), llegeix_import(preu_text))

    def import_net(self):
        return self.preu * self.quantitat
