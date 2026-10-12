import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

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
      home: const MyHomePage(title: 'Mini Imgur'),
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

  @override
  void initState() {
    super.initState();
    _loadImages();
  }

  Future<void> _loadImages() async {
    final response = await http.get(Uri.parse('http://localhost:80/images'));
    print(response.statusCode);
    print(response.body);
    final data = jsonDecode(response.body);
    setState(() {
      _images = data ?? [];
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,

        title: Text(widget.title),
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
                    'http://localhost:80/images/${img['id']}/thumbnail',
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
    );
  }
}
