+++
title = "Pipeline Reference"
description = "Complete reference for writing YAML pipeline files."
weight = 3
sort_by = "weight"
insert_anchor_links = "right"
+++

This section is the complete reference for authoring Spade pipelines. A pipeline is a YAML file that declares a series of block invocations and their dependencies, forming a directed acyclic graph (DAG) of processing steps.

If you are writing a pipeline by hand or generating one with an LLM, start with [Short Codes and Hand-Authoring](/pipelines/short-codes/) -- it shows how to use `@`-prefixed labels instead of UUIDs for block IDs.
