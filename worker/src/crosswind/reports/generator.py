"""HTML report generator for Crosswind evaluations."""

from datetime import datetime
from pathlib import Path
from typing import Any

import structlog
from jinja2 import Environment, FileSystemLoader, select_autoescape

from crosswind.storage.file_storage import FileStorage, create_file_storage

logger = structlog.get_logger()


class ReportGenerator:
    """Generates HTML reports from evaluation results."""

    def __init__(self, file_storage: FileStorage | None = None) -> None:
        self.file_storage = file_storage or create_file_storage()

        # Set up Jinja2 environment
        template_dir = Path(__file__).parent / "templates"
        self.env = Environment(
            loader=FileSystemLoader(template_dir),
            autoescape=select_autoescape(["html", "xml"]),
        )

        # Add custom filters
        self.env.filters["cos"] = lambda x: __import__("math").cos(x)
        self.env.filters["sin"] = lambda x: __import__("math").sin(x)

    async def generate_report(
        self,
        run_id: str,
        agent: dict[str, Any],
        eval_type: str,
        mode: str,
        summary_scores: dict[str, Any],
        regulatory_compliance: dict[str, Any],
        recommendations: list[dict[str, Any]],
        failures: list[dict[str, Any]],
        sample_passes: list[dict[str, Any]],
        threat_analysis: dict[str, Any] | None = None,
        trust_analysis: dict[str, Any] | None = None,
        performance_metrics: dict[str, Any] | None = None,
        started_at: datetime | None = None,
        completed_at: datetime | None = None,
    ) -> str:
        """Generate an HTML report and save it to storage.

        Returns the storage path of the generated report.
        """
        log = logger.bind(run_id=run_id, eval_type=eval_type)
        log.info("Generating HTML report")

        # Prepare template context
        context = self._build_context(
            run_id=run_id,
            agent=agent,
            eval_type=eval_type,
            mode=mode,
            summary_scores=summary_scores,
            regulatory_compliance=regulatory_compliance,
            recommendations=recommendations,
            failures=failures,
            sample_passes=sample_passes,
            threat_analysis=threat_analysis,
            trust_analysis=trust_analysis,
            performance_metrics=performance_metrics,
            started_at=started_at,
            completed_at=completed_at,
        )

        # Render template
        template = self.env.get_template("report.html")
        html_content = template.render(**context)

        # Save to storage
        report_path = f"reports/{run_id}/report.html"
        await self.file_storage.upload(
            path=report_path,
            content=html_content.encode("utf-8"),
            content_type="text/html",
        )

        log.info("HTML report generated", path=report_path)
        return report_path

    def _build_context(
        self,
        run_id: str,
        agent: dict[str, Any],
        eval_type: str,
        mode: str,
        summary_scores: dict[str, Any],
        regulatory_compliance: dict[str, Any],
        recommendations: list[dict[str, Any]],
        failures: list[dict[str, Any]],
        sample_passes: list[dict[str, Any]],
        threat_analysis: dict[str, Any] | None = None,
        trust_analysis: dict[str, Any] | None = None,
        performance_metrics: dict[str, Any] | None = None,
        started_at: datetime | None = None,
        completed_at: datetime | None = None,
    ) -> dict[str, Any]:
        """Build the template context from evaluation data."""
        overall_score = summary_scores.get("overall", 0)
        category_scores = summary_scores.get("byCategory", {})
        severity_scores = summary_scores.get("bySeverity", {})
        refusal_analysis = summary_scores.get("refusalAnalysis")
        asr = summary_scores.get("asr", {})

        # Calculate totals
        total_tests = sum(
            1 for _ in failures
        ) + sum(1 for _ in sample_passes)

        # For more accurate totals, use category breakdown if available
        if trust_analysis and trust_analysis.get("byQualityDimension"):
            total_tests = sum(
                dim.get("total", 0)
                for dim in trust_analysis["byQualityDimension"].values()
            )
            total_passed = sum(
                dim.get("passed", 0)
                for dim in trust_analysis["byQualityDimension"].values()
            )
        else:
            # Estimate from ASR for red_team
            if asr:
                total_tests = asr.get("full", 0) + asr.get("partial", 0) + asr.get("blocked", 0)
                total_passed = asr.get("blocked", 0)
            else:
                total_passed = len(sample_passes)

        # Calculate critical issues count
        critical_issues = sum(
            1 for f in failures
            if f.get("severity") == "critical"
        )

        # Count categories
        categories_tested = len(category_scores)
        prompts_per_category = total_tests / categories_tested if categories_tested > 0 else 0

        # Determine score interpretation
        if overall_score >= 0.9:
            score_interpretation = "Excellent"
        elif overall_score >= 0.8:
            score_interpretation = "Strong"
        elif overall_score >= 0.6:
            score_interpretation = "Moderate"
        elif overall_score >= 0.4:
            score_interpretation = "Needs Improvement"
        else:
            score_interpretation = "Critical Attention Required"

        # Build key finding
        key_finding = self._generate_key_finding(
            eval_type=eval_type,
            overall_score=overall_score,
            failures=failures,
            threat_analysis=threat_analysis,
            trust_analysis=trust_analysis,
        )

        # Calculate duration
        duration = "N/A"
        if performance_metrics and performance_metrics.get("totalDurationSeconds"):
            secs = performance_metrics["totalDurationSeconds"]
            if secs >= 3600:
                duration = f"{secs / 3600:.1f}h"
            elif secs >= 60:
                duration = f"{secs / 60:.1f}m"
            else:
                duration = f"{secs:.0f}s"
        elif started_at and completed_at:
            delta = completed_at - started_at
            secs = delta.total_seconds()
            if secs >= 3600:
                duration = f"{secs / 3600:.1f}h"
            elif secs >= 60:
                duration = f"{secs / 60:.1f}m"
            else:
                duration = f"{secs:.0f}s"

        # Format dates
        eval_date = completed_at.strftime("%B %d, %Y") if completed_at else datetime.utcnow().strftime("%B %d, %Y")
        generated_at = datetime.utcnow().strftime("%Y-%m-%d %H:%M UTC")

        return {
            # Metadata
            "run_id": run_id,
            "agent_name": agent.get("name", "Unknown Agent"),
            "eval_type": eval_type,
            "eval_type_display": "Red Team (Security)" if eval_type == "red_team" else "Trust (Compliance)",
            "mode": mode.title(),
            "eval_date": eval_date,
            "generated_at": generated_at,
            "duration": duration,

            # Titles
            "title": f"{'Security' if eval_type == 'red_team' else 'Trust'} Evaluation Report",
            "subtitle": f"Comprehensive {'security' if eval_type == 'red_team' else 'trust and compliance'} assessment for {agent.get('name', 'your agent')}",

            # Summary scores
            "overall_score": overall_score,
            "score_interpretation": score_interpretation,
            "category_scores": category_scores,
            "severity_scores": severity_scores,
            "refusal_analysis": refusal_analysis,

            # ASR metrics (red_team)
            "asr_blocked": asr.get("blocked", 0),
            "asr_blocked_pct": (asr.get("blocked", 0) / total_tests * 100) if total_tests > 0 else 0,

            # Totals
            "total_tests": total_tests,
            "total_passed": total_passed,
            "critical_issues": critical_issues,
            "categories_tested": categories_tested,
            "prompts_per_category": prompts_per_category,

            # Key finding
            "key_finding": key_finding,

            # Analysis
            "threat_analysis": threat_analysis,
            "trust_analysis": trust_analysis,
            "regulatory_compliance": regulatory_compliance,

            # Details
            "recommendations": recommendations,
            "failures": failures,
            "sample_passes": sample_passes,
        }

    def _generate_key_finding(
        self,
        eval_type: str,
        overall_score: float,
        failures: list[dict[str, Any]],
        threat_analysis: dict[str, Any] | None = None,
        trust_analysis: dict[str, Any] | None = None,
    ) -> str:
        """Generate a key finding summary."""
        if not failures:
            if overall_score >= 0.9:
                return f"Agent demonstrates strong {'security' if eval_type == 'red_team' else 'trust'} posture with no significant vulnerabilities detected."
            return f"Agent passed all tests with an overall score of {overall_score * 100:.0f}%."

        # For red_team, analyze threat patterns
        if eval_type == "red_team" and threat_analysis:
            vulnerable_threats = [
                (tid, t)
                for tid, t in threat_analysis.get("byOwaspAsi", {}).items()
                if t.get("successRate", 0) > 0
            ]
            if vulnerable_threats:
                top_threat = max(vulnerable_threats, key=lambda x: x[1].get("successRate", 0))
                return f"Agent is resilient to most attacks but shows vulnerability to {top_threat[1].get('threatName', top_threat[0])} with {top_threat[1].get('successRate', 0) * 100:.1f}% attack success rate."

        # For trust, analyze dimension patterns
        if eval_type == "trust" and trust_analysis:
            top_issues = trust_analysis.get("topIssues", [])
            if top_issues:
                worst = top_issues[0]
                return f"Agent shows significant gaps in {worst.get('dimensionName', 'quality')} ({worst.get('failureRate', 0) * 100:.0f}% failure rate), which may impact regulatory compliance."

        # Generic finding based on failure categories
        failure_categories = {}
        for f in failures:
            cat = f.get("category", "unknown")
            failure_categories[cat] = failure_categories.get(cat, 0) + 1

        if failure_categories:
            top_category = max(failure_categories.items(), key=lambda x: x[1])
            return f"Agent shows weakness in {top_category[0].replace('_', ' ')} category with {top_category[1]} failure(s). Review recommendations for remediation steps."

        return f"Agent achieved {overall_score * 100:.0f}% score with {len(failures)} identified issues requiring attention."
