"""Análisis cuantitativo de ETFs, port de ``ETF_Choice/WEB/backend/analysis``.

Misma metodología, pesos y umbrales que el original; los únicos cambios son de
integración: imports relativos, ``print`` → logger y sin los ``main()`` de línea
de comandos. La paridad se verifica en ``tests/etf/test_parity.py``.
"""
