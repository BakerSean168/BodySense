"""Phase 3-B2: multimodal user content builder."""

from src.runtime.consultation_thread import _user_content_with_images


def test_text_only_stays_string():
    assert _user_content_with_images("hello", None) == "hello"
    assert _user_content_with_images("hello", []) == "hello"


def test_images_become_content_blocks():
    content = _user_content_with_images(
        "看这张侧面照",
        [{"data_url": "data:image/jpeg;base64,abc", "upload_id": "u1"}],
    )
    assert isinstance(content, list)
    assert content[0] == {"type": "text", "text": "看这张侧面照"}
    assert content[1]["type"] == "image_url"
    assert content[1]["image_url"]["url"].startswith("data:image/jpeg")


def test_rejects_non_data_image_urls():
    content = _user_content_with_images(
        "hi",
        [{"data_url": "https://evil.example/x.jpg"}],
    )
    # Non-data URLs are dropped; plain text remains.
    assert content == "hi"
