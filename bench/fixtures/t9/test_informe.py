import unittest
from decimal import Decimal

from factures.model import Linia
from factures.informe import total_factura


class TestInforme(unittest.TestCase):
    def test_factura_petita(self):
        linies = [Linia.de_text("cafè", 2, "1,50 €"), Linia.de_text("pa", 1, "2,00")]
        self.assertEqual(total_factura(linies), (Decimal("5.00"), Decimal("1.05"), Decimal("6.05")))

    def test_factura_gran(self):
        linies = [Linia.de_text("portàtil", 1, "1.234,56 €"), Linia.de_text("funda", 2, "19,90 €")]
        self.assertEqual(total_factura(linies), (Decimal("1274.36"), Decimal("267.62"), Decimal("1541.98")))


if __name__ == "__main__":
    unittest.main()
