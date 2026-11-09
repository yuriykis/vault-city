package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/joho/godotenv"
)

const (
	screenWidth  = 800
	screenHeight = 450
	eyeHeight    = 1.7 // below the lowest roof (minHeight), so the aim ray hits the wall in front
	False        = 0
	True         = 1

	// raylib's default font only has Latin-1, so Polish and Cyrillic note
	// names are drawn with the system Arial.
	labelFontPath = "/System/Library/Fonts/Supplemental/Arial.ttf"
	labelFontSize = 20
	labelPadding  = 6.0               // pixels between a label's text and its background edge
	labelLineStep = labelFontSize + 4 // pixels from one label line to the next
	labelMaxWidth = screenWidth - 2*labelMargin - 2*labelPadding
)

func main() {
	// .env is optional and never overrides variables already set in the shell.
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("read .env: %v", err)
	}
	vaultPath := flag.String("vault", os.Getenv("VAULT_PATH"),
		"Obsidian vault to read, never written (default $VAULT_PATH)")
	flag.Parse()
	if *vaultPath == "" {
		log.Fatal("no vault: set VAULT_PATH in .env (see .env.example) or pass -vault")
	}

	vault, err := LoadVault(os.DirFS(*vaultPath))
	if err != nil {
		log.Fatalf("read vault %q: %v", *vaultPath, err)
	}
	for _, w := range vault.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	fmt.Println("Vault:", *vaultPath)
	printSummary(os.Stdout, vault)

	city := BuildCity(vault.Notes)
	tiles := groundTiles(city.Ground())

	rl.InitWindow(screenWidth, screenHeight, "raylib [core] example - 3d camera first person")

	// Start on the street in front of the largest district, facing its hub.
	camera := rl.Camera3D{}
	camera.Position = rl.Vector3Add(city.Spawn, rl.NewVector3(0, eyeHeight, 0))
	camera.Target = rl.Vector3Add(camera.Position, city.Facing)
	camera.Up = rl.NewVector3(0.0, 1.0, 0.0) // Camera up vector (rotation towards target)
	camera.Fovy = 60.0                       // Camera field-of-view Y
	camera.Projection = rl.CameraPerspective // Camera projection type

	cameraMode := rl.CameraFirstPerson

	font, fontLoaded := rl.GetFontDefault(), false
	if _, err := os.Stat(labelFontPath); err != nil {
		fmt.Fprintln(os.Stderr, "warning: names may lose letters, no label font:", err)
	} else {
		font, fontLoaded = rl.LoadFontEx(labelFontPath, labelFontSize, labelRunes(vault.Notes)), true
	}
	labelWidth := func(s string) float32 { return rl.MeasureTextEx(font, s, labelFontSize, 1).X }
	showText := false // the label shows the note's name, or its text

	rl.DisableCursor()  // Limit cursor to relative movement inside the window
	rl.SetTargetFPS(60) // Set our game to run at 60 frames-per-second

	// Main game loop
	for !rl.WindowShouldClose() { // Detect window close button or ESC key
		// Switch camera mode
		if rl.IsKeyPressed(rl.KeyOne) {
			cameraMode = rl.CameraFree
			camera.Up = rl.NewVector3(0.0, 1.0, 0.0) // Reset roll
		}
		if rl.IsKeyPressed(rl.KeyTwo) {
			cameraMode = rl.CameraFirstPerson
			camera.Up = rl.NewVector3(0.0, 1.0, 0.0) // Reset roll
		}
		if rl.IsKeyPressed(rl.KeyThree) {
			cameraMode = rl.CameraThirdPerson
			camera.Up = rl.NewVector3(0.0, 1.0, 0.0) // Reset roll
		}
		if rl.IsKeyPressed(rl.KeyFour) {
			cameraMode = rl.CameraOrbital
			camera.Up = rl.NewVector3(0.0, 1.0, 0.0) // Reset roll
		}

		// Switch camera projection
		if rl.IsKeyPressed(rl.KeyP) {
			cameraMode = rl.CameraThirdPerson
			switch camera.Projection {
			case rl.CameraPerspective:
				// Create isometric view
				// Note: The target distance is related to the render distance in the orthographic projection
				camera.Position = rl.NewVector3(0.0, 2.0, -100.0)
				camera.Target = rl.NewVector3(0.0, 2.0, 0.0)
				camera.Up = rl.NewVector3(0.0, 1.0, 0.0)
				camera.Projection = rl.CameraOrthographic
				camera.Fovy = 20.0 // near plane width in CAMERA_ORTHOGRAPHIC
				rl.CameraYaw(&camera, -135*rl.Deg2rad, True)
				rl.CameraPitch(&camera, -45*rl.Deg2rad, True, True, False)
			case rl.CameraOrthographic:
				// Reset to default view
				camera.Position = rl.NewVector3(0.0, 2.0, 10.0)
				camera.Target = rl.NewVector3(0.0, 2.0, 0.0)
				camera.Up = rl.NewVector3(0.0, 1.0, 0.0)
				camera.Projection = rl.CameraPerspective
				camera.Fovy = 60.0
			}
		}

		// Switch the label between the note's name and its text. In free mode
		// Space also flies up.
		if rl.IsKeyPressed(rl.KeySpace) {
			showText = !showText
		}

		// Update camera computes movement internally depending on the camera mode.
		// Some default standard keyboard/mouse inputs are hardcoded to simplify use.
		// For advance camera controls, it's recommended to compute camera movement manually.
		rl.UpdateCamera(&camera, cameraMode) // Update camera

		// Buildings are solid for whoever walks: the camera in first person,
		// the player cube at the target in third person. Free and orbital
		// modes fly through everything.
		switch cameraMode {
		case rl.CameraFirstPerson:
			keepOut(&camera.Position, &camera.Target, city.Buildings)
		case rl.CameraThirdPerson:
			keepOut(&camera.Target, &camera.Position, city.Buildings)
		}

		/*
		   // Camera PRO usage example (EXPERIMENTAL)
		   // This new camera function allows custom movement/rotation values to be directly provided
		   // as input parameters, with this approach, rcamera module is internally independent of raylib inputs
		   UpdateCameraPro(&camera,
		       (Vector3){
		           (IsKeyDown(KEY_W) || IsKeyDown(KEY_UP))*0.1f -      // Move forward-backward
		           (IsKeyDown(KEY_S) || IsKeyDown(KEY_DOWN))*0.1f,
		           (IsKeyDown(KEY_D) || IsKeyDown(KEY_RIGHT))*0.1f -   // Move right-left
		           (IsKeyDown(KEY_A) || IsKeyDown(KEY_LEFT))*0.1f,
		           0.0f                                                // Move up-down
		       },
		       (Vector3){
		           GetMouseDelta().x*0.05f,                            // Rotation: yaw
		           GetMouseDelta().y*0.05f,                            // Rotation: pitch
		           0.0f                                                // Rotation: roll
		       },
		       GetMouseWheelMove()*2.0f);                              // Move to target (zoom)
		*/

		rl.BeginDrawing()

		rl.ClearBackground(skyColor)

		rl.BeginMode3D(camera)

		// Everything fades into the sky with distance (gstack-shortcut(dec-5227f913)).
		for _, t := range tiles {
			size := rl.Vector3Subtract(t.Max, t.Min)
			center := rl.Vector3Add(t.Min, rl.Vector3Scale(size, 0.5))
			rl.DrawPlane(center, rl.NewVector2(size.X, size.Z), fogColor(rl.LightGray, distanceTo(camera.Position, t)))
		}

		// One box per note; the wires keep edges visible without lighting.
		for _, b := range city.Buildings {
			size := rl.Vector3Subtract(b.Box.Max, b.Box.Min)
			center := rl.Vector3Add(b.Box.Min, rl.Vector3Scale(size, 0.5))
			distance := distanceTo(camera.Position, b.Box)
			rl.DrawCubeV(center, size, fogColor(b.Color, distance))
			rl.DrawCubeWiresV(center, size, fogColor(rl.DarkGray, distance))
		}

		// Draw player cube
		if cameraMode == rl.CameraThirdPerson {
			rl.DrawCube(camera.Target, 0.5, 0.5, 0.5, rl.Purple)
			rl.DrawCubeWires(camera.Target, 0.5, 0.5, 0.5, rl.DarkPurple)
		}

		rl.EndMode3D()

		// Name the building in the middle of the screen, at the point you aim
		// at, or show the start of its note.
		ray := rl.GetScreenToWorldRay(rl.NewVector2(screenWidth/2, screenHeight/2), camera)
		if b, at, ok := pick(ray, city.Buildings); ok {
			lines := []string{b.Name}
			if showText {
				lines = wrapText(cmp.Or(b.Text, "(empty note)"), labelMaxWidth, labelLines, labelWidth)
			}
			size := rl.NewVector2(0, float32(len(lines)-1)*labelLineStep+labelFontSize)
			for _, line := range lines {
				size.X = max(size.X, labelWidth(line))
			}
			pos := labelPosition(rl.GetWorldToScreen(at, camera), size, screenWidth, screenHeight)
			pad := rl.NewVector2(labelPadding, labelPadding)
			rl.DrawRectangleV(rl.Vector2Subtract(pos, pad), rl.Vector2Add(size, rl.Vector2Scale(pad, 2)), rl.Fade(rl.Black, 0.6))
			for i, line := range lines {
				rl.DrawTextEx(font, line, rl.NewVector2(pos.X, pos.Y+float32(i)*labelLineStep), labelFontSize, 1, rl.White)
			}
		}

		rl.DrawFPS(10, screenHeight-25)

		rl.EndDrawing()
	}

	if fontLoaded {
		rl.UnloadFont(font) // while the window's GPU context still exists
	}
	rl.CloseWindow()
}
