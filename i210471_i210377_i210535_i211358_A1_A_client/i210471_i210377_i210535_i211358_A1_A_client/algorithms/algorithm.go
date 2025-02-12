package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Point struct {
	ID               int
	Features         []float64
	OriginalFeatures []float64
	Cluster          int
	Visited          bool
}

type ClusteringResult struct {
	NumClusters  int            `json:"num_clusters"`
	NoisePoints  int            `json:"noise_points"`
	ClusterSizes map[int]int    `json:"cluster_sizes"`
	Parameters   DBSCANParams   `json:"parameters"`
}

type DBSCANParams struct {
	Eps      float64                              `json:"eps"`
	MinPts   int                                  `json:"min_pts"`
	Distance func([]float64, []float64) float64   `json:"-"`
}

type FeatureStats struct {
	Min    float64
	Max    float64
	Mean   float64
	StdDev float64
}

// FileReader interface defines the contract for different file format readers
type FileReader interface {
	ReadData() ([]Point, error)
}

// BaseReader contains common functionality for all readers
type BaseReader struct {
	filePath string
}

// Helper function to create appropriate reader based on file extension
func NewDataReader(filePath string) (FileReader, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".csv":
		return &CSVReader{BaseReader{filePath}}, nil
	case ".data":
		return &DataReader{BaseReader{filePath}}, nil
	case ".arff":
		return &ARFFReader{BaseReader{filePath}}, nil
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

// Calculate statistics for each feature
func calculateFeatureStats(data []Point) []FeatureStats {
	if len(data) == 0 || len(data[0].Features) == 0 {
		return nil
	}

	numFeatures := len(data[0].Features)
	stats := make([]FeatureStats, numFeatures)

	// Initialize stats
	for i := range stats {
		stats[i] = FeatureStats{
			Min: math.MaxFloat64,
			Max: -math.MaxFloat64,
		}
	}

	// Calculate min, max, and mean
	for _, point := range data {
		for i, value := range point.Features {
			stats[i].Min = math.Min(stats[i].Min, value)
			stats[i].Max = math.Max(stats[i].Max, value)
			stats[i].Mean += value
		}
	}

	// Finalize means
	for i := range stats {
		stats[i].Mean /= float64(len(data))
	}

	// Calculate standard deviation
	for _, point := range data {
		for i, value := range point.Features {
			diff := value - stats[i].Mean
			stats[i].StdDev += diff * diff
		}
	}

	for i := range stats {
		stats[i].StdDev = math.Sqrt(stats[i].StdDev / float64(len(data)))
	}

	return stats
}

// Normalize the dataset
func normalizeData(data []Point) []Point {
	stats := calculateFeatureStats(data)
	
	for i := range data {
		// Store original features
		data[i].OriginalFeatures = make([]float64, len(data[i].Features))
		copy(data[i].OriginalFeatures, data[i].Features)
		
		// Normalize features
		for j := range data[i].Features {
			if stats[j].Max > stats[j].Min {
				data[i].Features[j] = (data[i].Features[j] - stats[j].Min) / (stats[j].Max - stats[j].Min)
			} else {
				data[i].Features[j] = 0
			}
		}
	}
	return data
}

func parseFeatures(fields []string) ([]float64, error) {
	features := make([]float64, 0, len(fields))
	
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || field == "?" {
			continue
		}
		
		value, err := strconv.ParseFloat(field, 64)
		if err != nil {
			continue
		}
		features = append(features, value)
	}
	
	if len(features) == 0 {
		return nil, fmt.Errorf("no valid numeric features found")
	}
	
	return features, nil
}

// CSVReader implements FileReader for CSV files
type CSVReader struct {
	BaseReader
}

func (r *CSVReader) ReadData() ([]Point, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	
	var data []Point
	id := 0
	
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV header: %v", err)
	}
	
	if features, err := parseFeatures(header); err == nil {
		data = append(data, Point{
			ID:       id,
			Features: features,
			Cluster:  0,
		})
		id++
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV record: %v", err)
		}

		features, err := parseFeatures(record)
		if err != nil {
			continue
		}

		data = append(data, Point{
			ID:       id,
			Features: features,
			Cluster:  0,
		})
		id++
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no valid data points found in CSV file")
	}

	return data, nil
}

// DataReader implements FileReader for .data files
type DataReader struct {
	BaseReader
}

func (r *DataReader) ReadData() ([]Point, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening .data file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var data []Point
	id := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var fields []string
		if strings.Contains(line, ",") {
			fields = strings.Split(line, ",")
		} else {
			fields = strings.Fields(line)
		}

		features, err := parseFeatures(fields)
		if err != nil {
			continue
		}

		data = append(data, Point{
			ID:       id,
			Features: features,
			Cluster:  0,
		})
		id++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .data file: %v", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no valid data points found in .data file")
	}

	return data, nil
}

// ARFFReader implements FileReader for ARFF files
type ARFFReader struct {
	BaseReader
}

func (r *ARFFReader) ReadData() ([]Point, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening ARFF file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var data []Point
	id := 0
	dataSection := false
	var numericAttributes []bool
	
	for scanner.Scan() {
		line := strings.TrimSpace(strings.ToLower(scanner.Text()))
		
		if line == "" || strings.HasPrefix(line, "%") {
			continue
		}
		
		if strings.HasPrefix(line, "@data") {
			dataSection = true
			break
		}
		
		if strings.HasPrefix(line, "@attribute") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				attributeType := fields[2]
				isNumeric := attributeType == "numeric" || 
					     attributeType == "real" || 
					     attributeType == "integer"
				numericAttributes = append(numericAttributes, isNumeric)
			}
		}
	}
	
	if !dataSection {
		return nil, fmt.Errorf("no @DATA section found in ARFF file")
	}
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" || strings.HasPrefix(line, "%") {
			continue
		}
		
		fields := strings.Split(line, ",")
		var numericFeatures []float64
		
		for i, field := range fields {
			if i >= len(numericAttributes) || !numericAttributes[i] {
				continue
			}
			
			field = strings.TrimSpace(field)
			if field == "?" {
				continue
			}
			
			value, err := strconv.ParseFloat(field, 64)
			if err != nil {
				continue
			}
			numericFeatures = append(numericFeatures, value)
		}
		
		if len(numericFeatures) > 0 {
			data = append(data, Point{
				ID:       id,
				Features: numericFeatures,
				Cluster:  0,
			})
			id++
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading ARFF file: %v", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no valid data points found in ARFF file")
	}

	return data, nil
}

// Calculate optimal eps using k-distance graph
func calculateOptimalEps(data []Point) float64 {
	k := 4
	distances := make([]float64, len(data))
	
	for i := range data {
		neighborDists := make([]float64, 0)
		for j := range data {
			if i != j {
				dist := EuclideanDistance(data[i].Features, data[j].Features)
				neighborDists = append(neighborDists, dist)
			}
		}
		sort.Float64s(neighborDists)
		if len(neighborDists) >= k {
			distances[i] = neighborDists[k-1]
		}
	}
	
	sort.Float64s(distances)
	index := int(float64(len(distances)) * 0.9)
	return distances[index]
}

// Calculate optimal MinPts based on dataset size
func calculateOptimalMinPts(numPoints int) int {
	minPts := int(math.Log2(float64(numPoints)))
	if minPts < 3 {
		return 3
	}
	if minPts > 10 {
		return 10
	}
	return minPts
}

func FindNeighbors(points []Point, pointIndex int, eps float64, distanceFunc func([]float64, []float64) float64) []int {
	var neighbors []int
	for i := range points {
		if i != pointIndex {
			distance := distanceFunc(points[pointIndex].Features, points[i].Features)
			if distance <= eps {
				neighbors = append(neighbors, i)
			}
		}
	}
	sort.Ints(neighbors)
	return neighbors
}

func ExpandCluster(points []Point, pointIndex int, neighbors []int, cluster int, params DBSCANParams) []Point {
	points[pointIndex].Cluster = cluster
	
	for i := 0; i < len(neighbors); i++ {
		neighborIndex := neighbors[i]
		
		if !points[neighborIndex].Visited {
			points[neighborIndex].Visited = true
			neighborNeighbors := FindNeighbors(points, neighborIndex, params.Eps, params.Distance)
			
			if len(neighborNeighbors) >= params.MinPts {
				for _, newNeighbor := range neighborNeighbors {
					found := false
					for _, existing := range neighbors {
						if newNeighbor == existing {
							found = true
							break
						}
					}
					if !found {
						neighbors = append(neighbors, newNeighbor)
					}
				}
			}
		}

		if points[neighborIndex].Cluster == 0 {
			points[neighborIndex].Cluster = cluster
		}
	}
	
	return points
}

func DBSCAN(points []Point) []Point {
	points = normalizeData(points)
	
	eps := calculateOptimalEps(points)
	minPts := calculateOptimalMinPts(len(points))
	
	params := DBSCANParams{
		Eps:      eps,
		MinPts:   minPts,
		Distance: EuclideanDistance,
	}
	
	fmt.Printf("Calculated parameters: eps=%.3f, minPts=%d\n", eps, minPts)
	
	cluster := 0
	
	sort.Slice(points, func(i, j int) bool {
		return points[i].Features[0] < points[j].Features[0]
	})
	
	for i := range points {
		if points[i].Visited {
			continue
		}
		
		points[i].Visited = true
		neighbors := FindNeighbors(points, i, params.Eps, params.Distance)
		
		if len(neighbors) < params.MinPts {
			points[i].Cluster = -1
		} else {
			cluster++
			points = ExpandCluster(points, i, neighbors, cluster, params)
		}
	}
	
	return points
}

func EuclideanDistance(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

func readDataFromFile(filePath string) ([]Point, error) {
	reader, err := NewDataReader(filePath)
	if err != nil {
		return nil, err
	}
	
	return reader.ReadData()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Error: Dataset path not provided")
		os.Exit(1)
	}

	datasetPath := os.Args[1]
	data, err := readDataFromFile(datasetPath)
	if err != nil {
		fmt.Printf("Error reading dataset: %v\n", err)
		os.Exit(1)
	}

	if len(data) == 0 {
		fmt.Println("Error: No valid data points found in the file")
		os.Exit(1)
	}
fmt.Println("HELLOOO!!!")
}
