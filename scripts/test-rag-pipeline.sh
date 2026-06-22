#!/bin/bash

# Test RAG Pipeline - End-to-End Test
# This script tests the complete RAG pipeline: embedding, retrieval, and reranking.

set -e

API_URL="${API_URL:-http://localhost:8080}"
AI_SERVICE_URL="${AI_SERVICE_URL:-http://localhost:8100}"

echo "🧪 Testing RAG Pipeline"
echo "========================"
echo "API URL: $API_URL"
echo "AI Service URL: $AI_SERVICE_URL"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function to check response
check_response() {
    local response="$1"
    local expected_status="$2"
    local test_name="$3"

    local status=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | sed '$d')

    if [ "$status" = "$expected_status" ]; then
        echo -e "${GREEN}✓ $test_name${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ $test_name (expected $expected_status, got $status)${NC}"
        echo "Response: $body"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Test 1: Health check
echo "📋 Test 1: Health Check"
response=$(curl -s -w "\n%{http_code}" "$API_URL/api/health")
check_response "$response" "200" "API health check"

response=$(curl -s -w "\n%{http_code}" "$AI_SERVICE_URL/health")
check_response "$response" "200" "AI Service health check"
echo ""

# Test 2: Add knowledge entries
echo "📝 Test 2: Add Knowledge Entries"

# Entry 1: Posture issues
response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/knowledge/entries" \
    -H "Content-Type: application/json" \
    -d '{
        "category": "posture",
        "title": "上交叉综合征（圆肩）",
        "content": "上交叉综合征是一种常见的体态问题，主要表现为圆肩、驼背、头前伸。常见于长时间使用电脑的人群。症状包括肩部疼痛、颈部僵硬、头痛等。改善方法包括拉伸胸大肌、加强背部肌肉训练。",
        "source_video": "https://example.com/video1",
        "source_timestamp": "10:30"
    }')
check_response "$response" "200" "Add entry: 上交叉综合征"

# Entry 2: Exercise advice
response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/knowledge/entries" \
    -H "Content-Type: application/json" \
    -d '{
        "category": "exercise",
        "title": "核心肌群训练",
        "content": "核心肌群包括腹直肌、腹斜肌、竖脊肌等。核心稳定性对体态至关重要。推荐动作：平板支撑、死虫式、鸟狗式。每个动作保持30秒，做3组。",
        "source_video": "https://example.com/video2",
        "source_timestamp": "5:20"
    }')
check_response "$response" "200" "Add entry: 核心肌群训练"

# Entry 3: Nutrition advice
response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/knowledge/entries" \
    -H "Content-Type: application/json" \
    -d '{
        "category": "nutrition",
        "title": "骨骼健康营养建议",
        "content": "维持骨骼健康需要充足的钙、维生素D和蛋白质。建议每天摄入1000mg钙，可通过牛奶、豆制品、绿叶蔬菜获取。维生素D可通过晒太阳或补充剂获取。",
        "source_video": "https://example.com/video3",
        "source_timestamp": "15:45"
    }')
check_response "$response" "200" "Add entry: 骨骼健康营养建议"

# Entry 4: Shoulder pain
response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/knowledge/entries" \
    -H "Content-Type: application/json" \
    -d '{
        "category": "pain",
        "title": "肩周炎的自我康复",
        "content": "肩周炎（冻结肩）表现为肩部疼痛和活动受限。常见于50岁左右人群。康复方法：钟摆运动、爬墙运动、毛巾拉伸。每天练习2-3次，每次15分钟。",
        "source_video": "https://example.com/video4",
        "source_timestamp": "8:15"
    }')
check_response "$response" "200" "Add entry: 肩周炎的自我康复"

# Entry 5: Lower back pain
response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/knowledge/entries" \
    -H "Content-Type: application/json" \
    -d '{
        "category": "pain",
        "title": "腰痛的常见原因和改善",
        "content": "久坐导致的腰痛很常见，通常与核心肌群无力、腰椎前凸过大有关。改善方法：加强核心训练、改善坐姿、定期站立活动。避免长时间保持同一姿势。",
        "source_video": "https://example.com/video5",
        "source_timestamp": "12:30"
    }')
check_response "$response" "200" "Add entry: 腰痛的常见原因和改善"

echo ""

# Test 3: Get knowledge base stats
echo "📊 Test 3: Knowledge Base Stats"
response=$(curl -s -w "\n%{http_code}" "$API_URL/api/knowledge/stats")
check_response "$response" "200" "Get stats"
echo ""

# Test 4: Semantic search
echo "🔍 Test 4: Semantic Search"

# Search for shoulder-related issues
response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/knowledge/search" \
    -H "Content-Type: application/json" \
    -d '{
        "query": "肩膀疼怎么办",
        "top_k": 5,
        "top_n": 3
    }')
check_response "$response" "200" "Search: 肩膀疼怎么办"

# Search for posture issues
response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/knowledge/search" \
    -H "Content-Type: application/json" \
    -d '{
        "query": "驼背圆肩如何改善",
        "top_k": 5,
        "top_n": 3
    }')
check_response "$response" "200" "Search: 驼背圆肩如何改善"

# Search for nutrition advice
response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/knowledge/search" \
    -H "Content-Type: application/json" \
    -d '{
        "query": "吃什么对骨骼好",
        "top_k": 5,
        "top_n": 3
    }')
check_response "$response" "200" "Search: 吃什么对骨骼好"

echo ""

# Test 5: Get single entry
echo "📖 Test 5: Get Single Entry"
response=$(curl -s -w "\n%{http_code}" "$API_URL/api/knowledge/entries/1")
check_response "$response" "200" "Get entry by ID"
echo ""

# Test 6: Delete entry
echo "🗑️ Test 6: Delete Entry"
response=$(curl -s -w "\n%{http_code}" -X DELETE "$API_URL/api/knowledge/entries/5")
check_response "$response" "200" "Delete entry"

# Verify deletion
response=$(curl -s -w "\n%{http_code}" "$API_URL/api/knowledge/entries/5")
check_response "$response" "404" "Verify entry deleted"
echo ""

# Summary
echo "========================"
echo "📊 Test Summary"
echo "========================"
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}❌ Some tests failed!${NC}"
    exit 1
fi
