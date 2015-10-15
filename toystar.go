package main

import (
	"fmt"
	"rundomizer/server/geo"
	"rundomizer/server/utils"
	"sync"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type WriteLockMap struct {
	sync.RWMutex
	m map[int64]PathNode
}

//PathNode is used for traversing with A star
type PathNode struct {
	GScore  float64
	HScore  float64
	FScore  float64
	GeoNode geo.Node
}

//Path is for something?
type Path struct {
	path []geo.Node
}

func main() {

	start := geo.ClosestNode(55.603765, 13.004886)
	goal := geo.ClosestNode(55.596479, 13.037026)
	fmt.Println("Starting A star")
	startTime := time.Now().UnixNano()
	aStar(start, goal)
	fmt.Println("A star: ", float64(time.Now().UnixNano()-startTime)/1000000000.0)
}

func aStar(start geo.Node, goal geo.Node) (float64, []geo.Node) {
	var database, databaseNr int64
	var totalPath Path
	closedSet := make(map[int64]PathNode)
	//Adding a write lock to buffer
	var buffer WriteLockMap
	buffer.m = make(map[int64]PathNode)
	openSet := make(map[int64]PathNode)
	cameFrom := make(map[int64]PathNode)

	session := utils.GetSession()
	defer session.Close()
	databaseNr++
	addToBuffer(&buffer, start, goal, session, &database)
	buffer.m[start.ID] = setScore(0.0, PathNode{0, start.DistanceTo(goal), 0, start})
	openSet[start.ID] = buffer.m[start.ID]

	for len(openSet) > 0 {
		current := smallestFScore(openSet)
		if current.GeoNode.ID == goal.ID {
			nano := 1000000000.0
			fmt.Println("Database fetch: ", float64(database)/nano, "s")
			fmt.Println("number of fetches: ", databaseNr)
			var distance float64
			distance, totalPath.path = reconstructPath(cameFrom, current)

			return distance, totalPath.path
		}

		delete(openSet, current.GeoNode.ID)
		closedSet[current.GeoNode.ID] = current

		for _, neighbourID := range current.GeoNode.Neighbours {
			//If it's already examined
			neighbour, isInBuffer := buffer.m[neighbourID]
			if !isInBuffer {
				databaseNr++
				addToBuffer(&buffer, current.GeoNode, goal, session, &database)
				neighbour = buffer.m[neighbourID]
			}

			if _, exists := closedSet[neighbourID]; exists {
				continue
			}
			tmpGScore := current.GScore + current.GeoNode.DistanceTo(neighbour.GeoNode)

			_, exists := openSet[neighbourID]

			if !exists || tmpGScore < neighbour.GScore {
				cameFrom[neighbourID] = current
				neighbour = setScore(tmpGScore, neighbour)
				if !exists {
					openSet[neighbourID] = neighbour
				}
			}
		}
	}
	return -1.0, []geo.Node{}
}

func smallestFScore(openSet map[int64]PathNode) PathNode {
	smallest := -1.0
	var smallestKey int64
	for key, val := range openSet {
		if smallest < 0 || val.GScore < smallest {
			smallest = val.GScore
			smallestKey = key
		}
	}
	return openSet[smallestKey]
}

func reconstructPath(cameFrom map[int64]PathNode, currentPathNode PathNode) (float64, []geo.Node) {
	gScore := currentPathNode.GScore
	current := currentPathNode.GeoNode
	var totalPath []geo.Node
	totalPath = append(totalPath, current)
	_, currentExists := cameFrom[current.ID]
	for currentExists {
		current = cameFrom[current.ID].GeoNode
		totalPath = append(totalPath, current)
		_, currentExists = cameFrom[current.ID]
	}
	totalPath = reversePath(totalPath)
	return gScore, totalPath
}

func reversePath(path []geo.Node) (reversedPath []geo.Node) {
	for i := (len(path) - 1); i >= 0; i-- {
		reversedPath = append(reversedPath, path[i])
	}
	return reversedPath
}

func setScore(gScore float64, node PathNode) PathNode {
	node.GScore = gScore
	node.FScore = node.HScore + node.GScore
	return node
}

func addToBuffer(buffer *WriteLockMap, at, goal geo.Node, session *mgo.Session, database *int64) {
	var wg sync.WaitGroup
	startTime := time.Now().UnixNano()
	//Threading
	width := at.To(goal) * 1
	latitudes := geo.CreateIntervals(at.Lat-width, at.Lat+width, 1)
	longitudes := geo.CreateIntervals(at.Lon-width, at.Lon+width, 4)
	wg.Add(len(latitudes) * len(longitudes))

	session.SetPoolLimit(4096)
	for _, latitude := range latitudes {
		for _, longitude := range longitudes {
			go addToBufferLong(buffer, session, &wg, goal, latitude.From, latitude.To, longitude.From, longitude.To)
		}
	}

	wg.Wait()
	*database += time.Now().UnixNano() - startTime
}

func addToBufferLong(buffer *WriteLockMap, session *mgo.Session, wg *sync.WaitGroup, goal geo.Node, lowerLat, higherLat, lowerLon, higherLon float64) {
	defer wg.Done()
	c := session.DB("skane").C("nodes")

	query := c.Find(bson.M{"lat": bson.M{"$gt": lowerLat, "$lt": higherLat}, "lon": bson.M{"$gt": lowerLon, "$lt": higherLon}})

	startTime := time.Now().UnixNano()
	iter := query.Iter()
	var node geo.Node
	for iter.Next(&node) {
		tmp := PathNode{0, node.DistanceTo(goal), 0, node}
		buffer.Lock()
		buffer.m[node.ID] = tmp
		buffer.Unlock()
	}
	loop := float64(time.Now().UnixNano()-startTime) / 1000000000.0
	fmt.Println("loop:", loop)
}
