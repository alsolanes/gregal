"""Tests ocults de t9: l'agent no els veu. Comproven el parser directament
amb formats que el test visible no cobreix, per caçar arreglos fets a mida
del test (p. ex. un cas especial per a "1.234,56")."""
import sys
import unittest
from decimal import Decimal

sys.path.insert(0, ".")
from factures.parser import llegeix_import  # noqa: E402


class TestParserOcult(unittest.TestCase):
    def test_formats(self):
        casos = {
            "12,50": "12.50",
            "12,50 €": "12.50",
            "-3,00": "-3.00",
            "0,99€": "0.99",
            "1.234,56 €": "1234.56",
            "12.345.678,90 €": "12345678.90",
            "  7 €": "7",
            "2.000": "2000",
        }
        for text, esperat in casos.items():
            with self.subTest(text=text):
                self.assertEqual(llegeix_import(text), Decimal(esperat))


if __name__ == "__main__":
    unittest.main()
