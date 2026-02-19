"""Semantic text chunking for extracted documents.

Splits extracted text into meaningful chunks along natural boundaries
(headings, paragraphs, sections) rather than hard character truncation.

Chunking strategy maximizes chunk size for efficiency - only splitting
when a section exceeds the configured threshold. Small sections are kept
intact to preserve semantic coherence.

Content-type aware:
- Markdown/PDF (from Docling): Split on heading boundaries, then paragraphs
- CSV/Excel: Split by sheet sections or row batches
- JSON: Split by top-level keys
- Plain text: Split on paragraph breaks, then sentences
"""

import logging
import re

logger = logging.getLogger(__name__)

# Default max chunk size in characters (~3000 tokens).
# Tuned to keep large coherent sections together while staying
# within reasonable LLM input sizes.
DEFAULT_MAX_CHUNK_SIZE = 12_000

# Overlap applied only when forcibly splitting mid-content
# (not at natural heading/section boundaries)
FORCED_SPLIT_OVERLAP = 150


class TextChunk:
    """A semantically meaningful text chunk."""

    __slots__ = ("text", "heading", "index", "char_count")

    def __init__(self, text: str, heading: str, index: int):
        self.text = text
        self.heading = heading
        self.index = index
        self.char_count = len(text)

    def to_dict(self) -> dict:
        return {
            "text": self.text,
            "heading": self.heading,
            "index": self.index,
            "charCount": self.char_count,
        }


class SemanticChunker:
    """Splits extracted text into semantic chunks.

    Maximizes chunk size — only splits when a section exceeds max_chunk_size.
    """

    def __init__(self, max_chunk_size: int = DEFAULT_MAX_CHUNK_SIZE):
        self.max_chunk_size = max_chunk_size

    def chunk(self, text: str, content_type: str) -> list[dict]:
        """Chunk text based on content type.

        Args:
            text: Extracted text content
            content_type: MIME type of the source file

        Returns:
            List of chunk dicts with keys: text, heading, index, charCount
        """
        if not text or not text.strip():
            return []

        # If text fits in a single chunk, return as-is
        if len(text) <= self.max_chunk_size:
            return [TextChunk(text=text, heading="", index=0).to_dict()]

        # Route to content-type-specific chunker
        if content_type in _STRUCTURED_TYPES:
            chunks = self._chunk_structured(text, content_type)
        elif content_type in _JSON_TYPES:
            chunks = self._chunk_json(text)
        else:
            # Markdown, PDF output, plain text, and everything else
            chunks = self._chunk_markdown(text)

        return [c.to_dict() for c in chunks]

    def _chunk_markdown(self, text: str) -> list[TextChunk]:
        """Split markdown text by headings, then by paragraphs if needed.

        Strategy:
        1. Split on top-level heading boundaries (# or ##)
        2. If a section still exceeds max size, split on sub-headings (###, ####)
        3. If still too large, split on paragraph breaks (double newline)
        4. Last resort: split on sentence boundaries with overlap
        """
        sections = self._split_on_headings(text)

        chunks: list[TextChunk] = []
        idx = 0

        for heading, body in sections:
            section_text = f"{heading}\n{body}".strip() if heading else body.strip()

            if not section_text:
                continue

            if len(section_text) <= self.max_chunk_size:
                chunks.append(TextChunk(text=section_text, heading=self._clean_heading(heading), index=idx))
                idx += 1
            else:
                # Section too large — split further
                sub_chunks = self._split_large_section(section_text, self._clean_heading(heading))
                for sc in sub_chunks:
                    sc.index = idx
                    chunks.append(sc)
                    idx += 1

        return chunks if chunks else [TextChunk(text=text, heading="", index=0)]

    def _split_on_headings(self, text: str) -> list[tuple[str, str]]:
        """Split text into (heading, body) pairs on # and ## boundaries.

        Returns a list of tuples where heading is the heading line (or "")
        and body is everything until the next heading.
        """
        # Match lines starting with # or ## (but not ### which is sub-heading)
        pattern = re.compile(r"^(#{1,2}\s+.+)$", re.MULTILINE)
        parts = pattern.split(text)

        sections: list[tuple[str, str]] = []

        if not parts:
            return [("", text)]

        # parts[0] is text before first heading (could be empty)
        # Then alternating: heading, body, heading, body, ...
        i = 0
        if not pattern.match(parts[0]):
            # Leading text before any heading
            if parts[0].strip():
                sections.append(("", parts[0]))
            i = 1

        while i < len(parts):
            heading = parts[i] if i < len(parts) else ""
            body = parts[i + 1] if i + 1 < len(parts) else ""
            sections.append((heading, body))
            i += 2

        return sections

    def _split_large_section(self, text: str, parent_heading: str) -> list[TextChunk]:
        """Split a large section further using sub-headings, then paragraphs."""
        # Try sub-headings (###, ####, etc.)
        sub_heading_pattern = re.compile(r"^(#{3,}\s+.+)$", re.MULTILINE)
        sub_parts = sub_heading_pattern.split(text)

        if len(sub_parts) > 1:
            # Has sub-headings — try grouping under them
            chunks = self._group_sub_sections(sub_parts, sub_heading_pattern, parent_heading)
            if chunks:
                return chunks

        # No sub-headings or still too large — split by paragraphs
        return self._split_by_paragraphs(text, parent_heading)

    def _group_sub_sections(
        self, parts: list[str], pattern: re.Pattern, parent_heading: str
    ) -> list[TextChunk]:
        """Group sub-heading sections, merging small ones together."""
        sections: list[tuple[str, str]] = []
        i = 0

        if not pattern.match(parts[0]):
            if parts[0].strip():
                sections.append(("", parts[0]))
            i = 1

        while i < len(parts):
            heading = parts[i] if i < len(parts) else ""
            body = parts[i + 1] if i + 1 < len(parts) else ""
            sections.append((heading, body))
            i += 2

        # Merge small adjacent sections to maximize chunk size
        chunks: list[TextChunk] = []
        current_text = ""
        current_heading = parent_heading

        for heading, body in sections:
            section_text = f"{heading}\n{body}".strip() if heading else body.strip()
            if not section_text:
                continue

            candidate = f"{current_text}\n\n{section_text}".strip() if current_text else section_text

            if len(candidate) <= self.max_chunk_size:
                current_text = candidate
                if not current_heading and heading:
                    current_heading = self._clean_heading(heading)
            else:
                # Flush current buffer
                if current_text:
                    chunks.append(TextChunk(text=current_text, heading=current_heading, index=0))

                # Start new buffer with this section
                if len(section_text) <= self.max_chunk_size:
                    current_text = section_text
                    current_heading = self._clean_heading(heading) or parent_heading
                else:
                    # Even a single sub-section is too large — split by paragraphs
                    para_chunks = self._split_by_paragraphs(section_text, self._clean_heading(heading) or parent_heading)
                    chunks.extend(para_chunks)
                    current_text = ""
                    current_heading = parent_heading

        if current_text:
            chunks.append(TextChunk(text=current_text, heading=current_heading, index=0))

        return chunks

    def _split_by_paragraphs(self, text: str, heading: str) -> list[TextChunk]:
        """Split text on paragraph breaks (double newline), merging small paragraphs."""
        paragraphs = re.split(r"\n\s*\n", text)
        paragraphs = [p.strip() for p in paragraphs if p.strip()]

        if not paragraphs:
            return [TextChunk(text=text, heading=heading, index=0)]

        chunks: list[TextChunk] = []
        current_text = ""

        for para in paragraphs:
            candidate = f"{current_text}\n\n{para}" if current_text else para

            if len(candidate) <= self.max_chunk_size:
                current_text = candidate
            else:
                # Flush current
                if current_text:
                    chunks.append(TextChunk(text=current_text, heading=heading, index=0))

                if len(para) <= self.max_chunk_size:
                    current_text = para
                else:
                    # Single paragraph exceeds limit — force split with overlap
                    forced = self._force_split(para, heading)
                    chunks.extend(forced)
                    current_text = ""

        if current_text:
            chunks.append(TextChunk(text=current_text, heading=heading, index=0))

        return chunks

    def _force_split(self, text: str, heading: str) -> list[TextChunk]:
        """Force-split a large block of text with overlap at sentence boundaries."""
        # Try sentence boundaries first
        sentences = re.split(r"(?<=[.!?])\s+", text)

        if len(sentences) <= 1:
            # No sentence boundaries — hard split with overlap
            return self._hard_split(text, heading)

        chunks: list[TextChunk] = []
        current_text = ""

        for sentence in sentences:
            candidate = f"{current_text} {sentence}" if current_text else sentence

            if len(candidate) <= self.max_chunk_size:
                current_text = candidate
            else:
                if current_text:
                    chunks.append(TextChunk(text=current_text, heading=heading, index=0))
                    # Add overlap from end of current chunk
                    overlap = current_text[-FORCED_SPLIT_OVERLAP:] if len(current_text) > FORCED_SPLIT_OVERLAP else ""
                    current_text = f"{overlap} {sentence}".strip() if overlap else sentence
                else:
                    # Single sentence exceeds limit
                    chunks.extend(self._hard_split(sentence, heading))
                    current_text = ""

        if current_text:
            chunks.append(TextChunk(text=current_text, heading=heading, index=0))

        return chunks

    def _hard_split(self, text: str, heading: str) -> list[TextChunk]:
        """Last resort: split at character boundary with overlap."""
        chunks: list[TextChunk] = []
        step = self.max_chunk_size - FORCED_SPLIT_OVERLAP
        pos = 0

        while pos < len(text):
            end = min(pos + self.max_chunk_size, len(text))
            chunks.append(TextChunk(text=text[pos:end], heading=heading, index=0))
            pos += step

        return chunks

    def _chunk_structured(self, text: str, content_type: str) -> list[TextChunk]:
        """Split structured text (CSV/Excel output) by sections."""
        # Excel output uses "=== Sheet: X ===" markers
        sheet_pattern = re.compile(r"^(=== Sheet: .+ ===)$", re.MULTILINE)
        parts = sheet_pattern.split(text)

        if len(parts) > 1:
            return self._group_sheet_sections(parts, sheet_pattern)

        # CSV or single-sheet — split by row batches
        return self._split_by_row_batches(text)

    def _group_sheet_sections(self, parts: list[str], pattern: re.Pattern) -> list[TextChunk]:
        """Group Excel sheet sections."""
        chunks: list[TextChunk] = []
        idx = 0
        current_text = ""
        current_heading = ""

        i = 0
        if not pattern.match(parts[0]):
            if parts[0].strip():
                current_text = parts[0].strip()
            i = 1

        while i < len(parts):
            heading = parts[i] if i < len(parts) else ""
            body = parts[i + 1].strip() if i + 1 < len(parts) else ""
            section_text = f"{heading}\n{body}" if body else heading
            i += 2

            candidate = f"{current_text}\n\n{section_text}".strip() if current_text else section_text

            if len(candidate) <= self.max_chunk_size:
                current_text = candidate
                if not current_heading:
                    current_heading = heading.strip("= ").strip()
            else:
                if current_text:
                    chunks.append(TextChunk(text=current_text, heading=current_heading, index=idx))
                    idx += 1

                if len(section_text) <= self.max_chunk_size:
                    current_text = section_text
                    current_heading = heading.strip("= ").strip()
                else:
                    batch_chunks = self._split_by_row_batches(section_text, heading.strip("= ").strip())
                    for bc in batch_chunks:
                        bc.index = idx
                        chunks.append(bc)
                        idx += 1
                    current_text = ""
                    current_heading = ""

        if current_text:
            chunks.append(TextChunk(text=current_text, heading=current_heading, index=idx))

        return chunks

    def _split_by_row_batches(self, text: str, heading: str = "") -> list[TextChunk]:
        """Split CSV/table text by row groups."""
        lines = text.split("\n")
        chunks: list[TextChunk] = []
        current_lines: list[str] = []
        current_len = 0
        idx = 0

        for line in lines:
            line_len = len(line) + 1  # +1 for newline

            if current_len + line_len > self.max_chunk_size and current_lines:
                chunk_text = "\n".join(current_lines)
                chunks.append(TextChunk(text=chunk_text, heading=heading, index=idx))
                idx += 1
                current_lines = []
                current_len = 0

            current_lines.append(line)
            current_len += line_len

        if current_lines:
            chunk_text = "\n".join(current_lines)
            chunks.append(TextChunk(text=chunk_text, heading=heading, index=idx))

        return chunks

    def _chunk_json(self, text: str) -> list[TextChunk]:
        """Split JSON text by top-level keys or array elements."""
        # For JSON, try splitting on top-level structure
        # If it's an object, split by top-level keys
        # If it's an array, split by elements
        # Fall back to paragraph-based splitting
        lines = text.split("\n")
        chunks: list[TextChunk] = []
        current_lines: list[str] = []
        current_len = 0
        idx = 0
        depth = 0

        for line in lines:
            stripped = line.strip()
            line_len = len(line) + 1

            # Track brace/bracket depth for top-level boundaries
            depth += stripped.count("{") + stripped.count("[")
            depth -= stripped.count("}") + stripped.count("]")

            current_lines.append(line)
            current_len += line_len

            # At top-level boundary (depth back to 0 or 1) and exceeding size
            if depth <= 1 and current_len > self.max_chunk_size and len(current_lines) > 1:
                chunk_text = "\n".join(current_lines)
                chunks.append(TextChunk(text=chunk_text, heading="", index=idx))
                idx += 1
                current_lines = []
                current_len = 0

        if current_lines:
            chunk_text = "\n".join(current_lines)
            if chunks and len(chunk_text) + chunks[-1].char_count <= self.max_chunk_size:
                # Merge small remainder into last chunk
                chunks[-1].text += "\n" + chunk_text
                chunks[-1].char_count = len(chunks[-1].text)
            else:
                chunks.append(TextChunk(text=chunk_text, heading="", index=idx))

        return chunks

    @staticmethod
    def _clean_heading(heading: str) -> str:
        """Strip markdown heading markers."""
        if not heading:
            return ""
        return re.sub(r"^#+\s*", "", heading.strip())


# Content types that use structured (row-based) chunking
_STRUCTURED_TYPES = {
    "text/csv",
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    "application/vnd.ms-excel",
}

_JSON_TYPES = {
    "application/json",
}
