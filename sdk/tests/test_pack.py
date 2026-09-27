"""
Tests for packaging and admission validation.
"""

from pathlib import Path
import tempfile
import zipfile
import pytest

from agentrix_training.pack import package_bot, validate_bot_directory


def test_package_trained_bot():
    bot_dir = Path("bots/trained_bot")
    assert bot_dir.is_dir()
    manifest = validate_bot_directory(bot_dir)
    assert manifest["name"] == "ApexTrainedAgent"
    assert manifest["runtime"] == "python-standard"

    with tempfile.TemporaryDirectory() as tmpdir:
        out_zip = Path(tmpdir) / "submission.zip"
        package_bot(bot_dir, out_zip, skip_admission=False)
        assert out_zip.is_file()

        with zipfile.ZipFile(out_zip, "r") as archive:
            names = archive.namelist()
            assert "agentrix.json" in names
            assert "agent.py" in names
            assert "weights.json" in names
