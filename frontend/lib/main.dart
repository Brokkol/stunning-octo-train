import 'dart:convert';
import 'dart:async';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:flutter/foundation.dart';
import 'package:permission_handler/permission_handler.dart';

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  // This widget is the root of your application.
  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'My service',
      theme: ThemeData(colorScheme: .fromSeed(seedColor: Colors.blue)),
      home: MyHomePage(title: 'Mini Imgur'),
    );
  }
}

class MyHomePage extends StatefulWidget {
  const MyHomePage({super.key, required this.title});

  final String title;

  @override
  State<MyHomePage> createState() => _MyHomePageState();
}

class _MyHomePageState extends State<MyHomePage> {
  List<dynamic> _images = [];
  Timer? _timer;
  String _baseUrl = '';

  Future<void> _loadSettings() async {
    final prefs = await SharedPreferences.getInstance();
    final saved = prefs.getString('base_url') ?? '';
    if (!mounted) return;
    setState(() => _baseUrl = saved);
    await _askNetworkPermission();
  }

  Future<void> _askNetworkPermission() async {
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      await Permission.accessLocalNetwork.request();
    }
  }

  Future<void> _uploadImage() async {
    final file = await FilePicker.pickFile(type: FileType.image);
    if (file == null) return; // отменил выбор

    final bytes = await file.readAsBytes();
    if (_baseUrl.isEmpty) return;
    final request = http.MultipartRequest(
      'POST',
      Uri.parse('$_baseUrl/images'),
    );
    request.files.add(
      http.MultipartFile.fromBytes('file', bytes, filename: file.name),
    );

    final response = await request.send();

    if (response.statusCode == 200 || response.statusCode == 201) {
      _loadImages();
    }
  }

  @override
  void initState() {
    super.initState();
    _loadSettings().then((_) => _loadImages());
    _timer = Timer.periodic(const Duration(seconds: 3), (_) => _loadImages());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _loadImages() async {
    if (_baseUrl.isEmpty) return;

    final response = await http.get(Uri.parse('$_baseUrl/images'));
    final data = jsonDecode(response.body);
    if (!mounted) return;
    setState(() {
      _images = data ?? [];
    });
  }

  Future<void> _editServer() async {
  final controller = TextEditingController(text: _baseUrl);
  final result = await showDialog<String>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('Server address'),
      content: TextField(
        controller: controller,
        decoration: const InputDecoration(hintText: 'http://192.168.68.54'),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(ctx),
          child: const Text('Cancel'),
        ),
        TextButton(
          onPressed: () => Navigator.pop(ctx, controller.text.trim()),
          child: const Text('Save'),
        ),
      ],
    ),
  );
  if (result == null) return; // нажал Cancel
  final prefs = await SharedPreferences.getInstance();
  await prefs.setString('base_url', result);
  setState(() => _baseUrl = result);
  _loadImages();
}

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,

        title: Text(widget.title),
        actions: [
          IconButton(icon: const Icon(Icons.settings), onPressed: _editServer),
        ],
      ),
      body: _images.isEmpty
          ? const Center(
              child: Text(
                'Unfortunately you have no pics at the moment',
                style: TextStyle(fontSize: 20, color: Colors.indigo),
              ),
            )
          : GridView.builder(
              itemCount: _images.length,
              itemBuilder: (context, i) {
                final img = _images[i];
                if (img['status'] == 'done') {
                  return Image.network(
                    '$_baseUrl/images/${img['id']}/thumbnail',
                    fit: BoxFit.cover,
                  );
                } else {
                  return ListTile(
                    title: Text(img['id']),
                    subtitle: Text(img['status']),
                  );
                }
              },
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 3,
              ),
            ),
      floatingActionButton: FloatingActionButton(
        tooltip: 'Upload', // used by assistive technologies
        onPressed: _uploadImage,
        child: Icon(Icons.add),
      ),
    );
  }
}
